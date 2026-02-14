package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

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
var logErr = log.New(os.Stderr, "err: ", 0)

func main() {
	var c Config
	flag.StringVar(&c.srcURL, "from", "", "Source server")
	flag.StringVar(&c.srcUsername, "fromuser", "", " Source username")
	flag.StringVar(&c.srcPassword, "fromp", "", "Source password")
	flag.StringVar(&c.srcFolder, "frombox", imap.InboxName, "Source folder")
	flag.StringVar(&c.dstURL, "to", "", "Destination server")
	flag.StringVar(&c.dstUsername, "touser", "", "Destination username")
	flag.StringVar(&c.dstPassword, "topw", "", "Destination password")
	flag.StringVar(&c.dstFolder, "tobox", "", "Destination [default: same as Source folder]")
	flag.BoolVar(&c.once, "once", false, "Only do this once")
	flag.BoolVar(&c.copy, "copy", false, "Copy messages instead of moving")
	flag.BoolVar(&c.verbose, "verbose", false, "Print verbose debug logs to stdout")
	flag.Parse()
	if c.dstFolder == "" {
		c.dstFolder = c.srcFolder
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
	dc, err := client.DialTLS(c.dstURL, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("dialing dst: %v", err)
	}
	defer dc.Logout()
	if err := dc.Login(c.dstUsername, c.dstPassword); err != nil {
		return fmt.Errorf("login to dst: %v", err)
	}
	// select the mailbox to check if it exists; if it does not, create it.
	if _, err := dc.Select(c.dstFolder, true); err != nil {
		if err := dc.Create(c.dstFolder); err != nil {
			return fmt.Errorf("creating dst mailbox: %v", err)
		}
	}

	uc, err := client.DialTLS(c.srcURL, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("dialing src: %v", err)
	}
	defer uc.Logout()
	if err := uc.Login(c.srcUsername, c.srcPassword); err != nil {
		return fmt.Errorf("login to src: %v", err)
	}
	mbox, err := uc.Select(c.srcFolder, c.copy)
	if err != nil {
		return fmt.Errorf("selecting src mailbox: %v", err)
	}

	updateCond := sync.Cond{
		L: &sync.Mutex{},
	}
	updates := make(chan client.Update, 64)
	updatesClose := make(chan struct{}, 1)
	uc.Updates = updates
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
			err := fetchForEach(uc, lastCountLocal+1, newCountLocal, items, func(msg *imap.Message) error {
				if c.verbose {
					logOut.Printf("appending message %v to dst", int(msg.SeqNum))
				}
				if err := dc.Append(c.dstFolder, msg.Flags, msg.InternalDate, msg.GetBody(bodySection)); err != nil {
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
			if err := remove(uc, removeIDs...); err != nil {
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
			errCh <- uc.Idle(idleCh, nil)
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
