package inbox

import (
	"log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type ImapProvider string

type Folder string

type Credentials struct {
	Username string
	Password string
}

const (
	GMX           ImapProvider = "imap.gmx.net:993"
	InboxFolder   Folder       = imap.InboxName
	GmxSpamFolder Folder       = "Spamverdacht"
	TrashFolder   Folder       = "Trash"
)

type Inbox struct {
	cred   *Credentials
	client *client.Client
}

// New creates a new Bot and authenticate with the given credentials.
func New(provider ImapProvider, cred *Credentials) (*Inbox, error) {
	inbox := new(Inbox)
	inbox.cred = cred

	// Connect to server
	client, err := client.DialTLS(string(provider), nil)
	if err != nil {
		return nil, err
	}

	err = client.Login(cred.Username, cred.Password)
	if err != nil {
		return nil, err
	}

	inbox.client = client

	return inbox, nil
}

// DeleteAllMessagesInFolder deletes all messages in the given folder.
// When expunge is set to "false", no "\DELETED" flag is set (safe mode). When set to "true", all messages removed permenantly.
func (i *Inbox) DeleteAllMessagesInFolder(expunge bool, folder Folder) error {
	mbox, err := selectFolder(i, folder)
	if err != nil {
		return err
	}

	delSeqSet := new(imap.SeqSet)
	delSeqSet.AddRange(1, mbox.Messages)

	if !expunge {
		return nil
	}

	return deleteMessagesPermanently(i, delSeqSet)
}

// DeleteMessagesInFolderFromAddress sets the "\DELETED" flag to all messages sent from the given addresses.
// When expunge is set to "false", no "\DELETED" flag is set (safe mode). When set to "true", messages matching to the given
// addresses are removed permenantly.
func (b *Inbox) DeleteMessagesInFolderFromAddress(expunge bool, folder Folder, addr ...string) error {
	mbox, err := selectFolder(b, folder)
	if err != nil {
		return err
	}

	errChan := make(chan error, 1)
	messages := make(chan *imap.Message, mbox.Messages)
	go func() {
		errChan <- fetchAllMessages(mbox, b, messages)
	}()

	delSeqSet := new(imap.SeqSet)

	compare(addr, messages, delSeqSet)

	if err := <-errChan; err != nil {
		return err
	}

	if !expunge {
		return nil
	}

	return deleteMessagesPermanently(b, delSeqSet)
}

// compare adds every message SeqNum sent from one of the given addresses to delSeqSet.
func compare(address []string, messages chan *imap.Message, delSeqSet *imap.SeqSet) {
	m := make(chan map[string]string, cap(address))
	for msg := range messages {
		go compareMessageWithAddresses(msg, address, m, delSeqSet)
	}

	close(m)

	printMessagesToDelete(m)
}

// printMessagesToDelete lists all messages for each address which will be deleted.
func printMessagesToDelete(msgMapChan chan map[string]string) {
	msgMap := make(map[string][]string)
	for m := range msgMapChan {
		for k := range m {
			msgMap[k] = append(msgMap[k], m[k])
		}
	}

	for x := range msgMap {
		log.Println("Messages to delete from", x+":")
		for _, y := range msgMap[x] {
			log.Println("\t", y)
		}
	}
}

// deleteMessagesPermanently sets the deleted flag and expunge them.
func deleteMessagesPermanently(b *Inbox, delSeqSet *imap.SeqSet) error {
	if err := b.client.Store(delSeqSet, imap.StoreItem(imap.AddFlags), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return err
	}

	return b.client.Expunge(nil)
}

// selectFolder sets the given folder as selected mailbox.
func selectFolder(b *Inbox, folder Folder) (*imap.MailboxStatus, error) {
	mbox, err := b.client.Select(string(folder), false)
	if err != nil {
		return nil, err
	}

	log.Println("Selected folder:", mbox.Name)

	return mbox, nil
}

func (b *Inbox) Logout() error {
	return b.client.Logout()
}

// fetchAllMessages fetches all messages in the selected mailbox.
func fetchAllMessages(mbox *imap.MailboxStatus, b *Inbox, messages chan *imap.Message) error {
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(1, mbox.Messages)
	if err := b.client.Fetch(seqSet, []imap.FetchItem{imap.FetchEnvelope}, messages); err != nil {
		return err
	}

	return nil
}

// compareMessageWithAddresses compares the given message address with the addresses to delete.
// The ID of a matching message is added to delSeqSet.
func compareMessageWithAddresses(msg *imap.Message, address []string, mapChan chan map[string]string, delSeqSet *imap.SeqSet) {
	m := make(map[string]string)
	for _, addr := range address {
		for _, from := range msg.Envelope.From {
			msgAddress := from.Address()
			if msgAddress == addr {
				m[addr] = msg.Envelope.Subject
				delSeqSet.AddNum(msg.SeqNum)
			}
		}
	}

	mapChan <- m
}

func main() {
	// Credentials
	cred := &Credentials{
		Username: "username",
		Password: "password",
	}

	// Create new inbox
	inbox, err := New(GMX, cred)
	if err != nil {
		log.Fatal(err)
	}

	// Delete all messages in the inbox folder
	if err := inbox.DeleteAllMessagesInFolder(true, InboxFolder); err != nil {
		log.Fatal(err)
	}

	// Delete all messages in the spam folder
	if err := inbox.DeleteAllMessagesInFolder(true, GmxSpamFolder); err != nil {
		log.Fatal(err)
	}

	// Delete all messages in the trash folder
	if err := inbox.DeleteAllMessagesInFolder(true, TrashFolder); err != nil {
		log.Fatal(err)
	}

	// Delete all messages from the given addresses in the inbox folder
	if err := inbox.DeleteMessagesInFolderFromAddress(true, InboxFolder, "address1", "address2"); err != nil {
		log.Fatal(err)
	}

	// Delete all messages from the given addresses in the spam folder
	if err := inbox.DeleteMessagesInFolderFromAddress(true, GmxSpamFolder, "address1", "address2"); err != nil {
		log.Fatal(err)
	}

	// Delete all messages from the given addresses in the trash folder
	if err := inbox.DeleteMessagesInFolderFromAddress(true, TrashFolder, "address1", "address2"); err != nil {
		log.Fatal(err)
	}

	// Logout
	if err := inbox.Logout(); err != nil {
		log.Fatal(err)
	}
}
