# mvimap
**Move (or copy) messages from IMAP to IMAP**

* Leveraging [go-imap](https://github.com/emersion/go-imap)
* Licence: MIT
* After: https://github.com/delthas/imap-forward
* Copying messages one-time or as a daemon (using IDLE for low-latency syncing)

## Usage
### Keep moving IMAP Inbox to IMAP Inbox
`mvimap -from example.com:993 -fromuser foo@example.com -frompw himom -to other.example.com:993 -touser bar@other.example.com -topw hidad`

### Move IMAP Inbox to IMAP Inbox Once
`mvimap -once -from example.com:993 -fromuser foo@example.com -frompw himom -to other.example.com:993 -touser bar@other.example.com -topw hidad`

### Keep copying IMAP Inbox to IMAP Inbox
`mvimap -copy -from example.com:993 -fromuser foo@example.com -frompw himom -to other.example.com:993 -touser bar@other.example.com -topw hidad`

### Copy IMAP Inbox to IMAP Inbox Once
`mvimap -copy -once -from example.com:993 -fromuser foo@example.com -frompw himom -to other.example.com:993 -touser bar@other.example.com -topw hidad`

