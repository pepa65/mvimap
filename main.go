package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

const version = "v0.1.0"
const name = "mvimap"

type Config struct {
	srcURL      string
	srcUsername string
	srcPassword string
	srcFolder   string
	dstURL      string
	dstUsername string
	dstPassword string
	dstFolder   string
	once        bool
	copy        bool
	verbose     bool
}

var logOut = log.New(os.Stdout, "", 0)
var logErr = log.New(os.Stderr, "Error: ", 0)

func main() {
	var c Config
	flag.StringVar(&c.srcFolder, "frombox", imap.InboxName, "Source folder [default: INBOX]")
	flag.StringVar(&c.dstFolder, "tobox", "", "Destination [default: same as Source folder]")
	flag.BoolVar(&c.once, "once", false, "Only do this once")
	flag.BoolVar(&c.copy, "copy", false, "Copy messages instead of moving")
	flag.BoolVar(&c.verbose, "verbose", false, "Print verbose debug logs to stdout")
	showVersion := flag.Bool("V", false, "Print version and exit")
	flag.BoolVar(showVersion, "version", false, "Print version and exit")
	flag.Usage = func() {
		logOut.Printf("%s %s - Move (or copy) messages from IMAP to IMAP\n", name, version)
		logOut.Printf("Usage:  %s [Options] FROM_SERVER FROM_USER FROM_PW TO_SERVER TO_USER TO_PW\n", name)
		logOut.Println("  If FROM_SERVER or TO_SERVER does not use port 993, then append :PORT")
		logOut.Println("  Options:")
		logOut.Println("    --once:              Only do this once [default: keep running every minute]")
		logOut.Println("    --copy:              Copy messages [default: move messages]")
		logOut.Println("    --verbose:           Print verbose debug logs to stdout")
		logOut.Println("    --frombox FROM_BOX:  The source folder [default: INBOX]")
		logOut.Println("    --tobox TO_BOX:      The destination folder [default: INBOX]")
		logOut.Println("    -h|--help:           Only show this help text")
		logOut.Println("    -V|--version:        Only show the app version")
	}
	flag.Parse()

	if *showVersion {
		logOut.Println(name, version)
		os.Exit(0)
	}
	if flag.NArg() != 6 {
		logErr.Printf("Need 6 arguments: FROM_SERVER FROM_USER FROM_PW TO_SERVER TO_USER TO_PW")
		os.Exit(1)
	}
	c.srcURL = flag.Arg(0)
	c.srcUsername = flag.Arg(1)
	c.srcPassword = flag.Arg(2)
	c.dstURL = flag.Arg(3)
	c.dstUsername = flag.Arg(4)
	c.dstPassword = flag.Arg(5)
	if c.dstFolder == "" {
		c.dstFolder = c.srcFolder
	}
	if strings.IndexByte(c.srcURL, ':') >= 0 {
		c.srcURL += ":993"
	}
	if strings.IndexByte(c.dstURL, ':') >= 0 {
		c.dstURL += ":993"
	}
	for {
		err := run(&c)
		if err != nil {
			logErr.Println(err)
		} else {
			break
		}
		time.Sleep(1 * time.Minute)
	}
}

func run(c *Config) error {
	to, err := client.DialTLS(c.dstURL, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("dialing dst: %v", err)
	}

	defer to.Logout()
	if err := to.Login(c.dstUsername, c.dstPassword); err != nil {
		return fmt.Errorf("login to dst: %v", err)
	}

	// select the mailbox to check if it exists; if it does not, create it.
	if _, err := to.Select(c.dstFolder, true); err != nil {
		if err := to.Create(c.dstFolder); err != nil {
			return fmt.Errorf("creating dst mailbox: %v", err)
		}
	}

	from, err := client.DialTLS(c.srcURL, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("dialing src: %v", err)
	}
	defer from.Logout()
	if err := from.Login(c.srcUsername, c.srcPassword); err != nil {
		return fmt.Errorf("login to src: %v", err)
	}

	mbox, err := from.Select(c.srcFolder, c.copy)
	if err != nil {
		return fmt.Errorf("selecting src mailbox: %v", err)
	}

	updateCond := sync.Cond{
		L: &sync.Mutex{},
	}
	updates := make(chan client.Update, 64)
	updatesClose := make(chan struct{}, 1)
	from.Updates = updates
	defer close(updatesClose)
	defer updateCond.Broadcast()
	lastCount := 0
	newCount := int(mbox.Messages)

	go func() {
		for {
			select {
			case update := <-updates:
				updateCond.L.Lock()
				switch update := update.(type) {
				case *client.MailboxUpdate:
					newCount = int(update.Mailbox.Messages)
					if c.verbose {
						logOut.Printf("adding messages: src now has %v messages", newCount)
					}
				case *client.ExpungeUpdate:
					lastCount--
					newCount--
					if c.verbose {
						logOut.Printf("removing message: src now has %v messages", newCount)
					}
				}
				updateCond.Broadcast()
				updateCond.L.Unlock()
			case <-updatesClose:
				return
			}
		}
	}()

	for {
		updateCond.L.Lock()
		newCountLocal := newCount
		lastCountLocal := lastCount
		updateCond.L.Unlock()
		if newCountLocal > lastCountLocal {
			if c.verbose {
				logOut.Printf("processing src messages %v to %v", lastCountLocal+1, newCountLocal)
			}
			bodySection := &imap.BodySectionName{
				Peek: true,
			}
			items := []imap.FetchItem{imap.FetchFlags, imap.FetchInternalDate, imap.FetchRFC822Size, imap.FetchEnvelope, imap.FetchBody, bodySection.FetchItem()}
			var removeIDs []int
			err := fetchForEach(from, lastCountLocal+1, newCountLocal, items, func(msg *imap.Message) error {
				if c.verbose {
					logOut.Printf("appending message %v to dst", int(msg.SeqNum))
				}
				if err := to.Append(c.dstFolder, msg.Flags, msg.InternalDate, msg.GetBody(bodySection)); err != nil {
					logErr.Printf("appending message to dst: %v", err)
					return nil
				}

				if !c.copy {
					removeIDs = append(removeIDs, int(msg.SeqNum))
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("fetching src messages: %v", err)
			}

			updateCond.L.Lock()
			lastCount = newCountLocal
			updateCond.L.Unlock()
			if err := remove(from, removeIDs...); err != nil {
				return fmt.Errorf("deleting src messages: %v", err)
			}

			if c.verbose {
				logOut.Printf("processed src messages, src now has %v messages", newCountLocal)
			}
		}
		if c.once {
			return nil
		}

		errCh := make(chan error, 1)
		updateCh := make(chan struct{}, 1)
		idleCh := make(chan struct{})

		go func() {
			errCh <- from.Idle(idleCh, nil)
		}()

		go func() {
			updateCond.L.Lock()
			updateCond.Wait()
			updateCond.L.Unlock()
			updateCh <- struct{}{}
		}()

		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		case <-updateCh:
		}
		close(idleCh)
	}
}

func fetchForEach(c *client.Client, start int, end int, fetchItems []imap.FetchItem, f func(message *imap.Message) error) error {
	var set imap.SeqSet
	set.AddRange(uint32(start), uint32(end))
	m := make(chan *imap.Message, end-start)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(&set, fetchItems, m)
	}()
	for msg := range m {
		if err := f(msg); err != nil {
			return err
		}
	}
	return <-done
}

func remove(c *client.Client, q ...int) error {
	if len(q) == 0 {
		return nil
	}
	var set imap.SeqSet
	for _, v := range q {
		set.AddNum(uint32(v))
	}
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.DeletedFlag}
	err := c.Store(&set, item, flags, nil)
	if err != nil {
		return err
	}
	err = c.Expunge(nil)
	return err
}
