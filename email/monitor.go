package email

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/emersion/go-imap"
)

type EmailMonitor struct {
	client    *IMAPClient
	interval  time.Duration
	lastCount uint32
	callback  func(*imap.Message)
	stopChan  chan bool
	mu        sync.Mutex
	running   bool
}

func NewEmailMonitor(client *IMAPClient, interval time.Duration, callback func(*imap.Message)) *EmailMonitor {
	return &EmailMonitor{
		client:   client,
		interval: interval,
		callback: callback,
		stopChan: make(chan bool),
	}
}

func (em *EmailMonitor) Start() error {
	em.mu.Lock()
	if em.running {
		em.mu.Unlock()
		return fmt.Errorf("monitor is already running")
	}
	em.running = true
	em.mu.Unlock()

	mbox, err := em.client.SelectMailbox("INBOX")
	if err != nil {
		em.mu.Lock()
		em.running = false
		em.mu.Unlock()
		return fmt.Errorf("failed to select mailbox: %v", err)
	}
	em.lastCount = mbox.Messages

	log.Printf("Email monitor started, initial message count: %d", em.lastCount)

	go em.monitorLoop()
	return nil
}

func (em *EmailMonitor) monitorLoop() {
	ticker := time.NewTicker(em.interval)
	defer ticker.Stop()

	for {
		select {
		case <-em.stopChan:
			return
		case <-ticker.C:
			em.checkForNewEmails()
		}
	}
}

func (em *EmailMonitor) checkForNewEmails() {
	mbox, err := em.client.SelectMailbox("INBOX")
	if err != nil {
		log.Printf("Failed to select mailbox: %v", err)
		return
	}

	currentCount := mbox.Messages
	if currentCount > em.lastCount {
		log.Printf("New emails detected: %d -> %d", em.lastCount, currentCount)

		newMessageCount := currentCount - em.lastCount
		newMessages, err := em.fetchNewMessages(em.lastCount+1, currentCount)
		if err != nil {
			log.Printf("Failed to fetch new messages: %v", err)
			return
		}

		for _, msg := range newMessages {
			if em.callback != nil {
				go em.callback(msg)
			}
		}

		em.lastCount = currentCount
		log.Printf("Processed %d new messages", newMessageCount)
	}
}

func (em *EmailMonitor) fetchNewMessages(from, to uint32) ([]*imap.Message, error) {
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		section := &imap.BodySectionName{}
		items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}
		done <- em.client.client.Fetch(seqset, items, messages)
	}()

	var result []*imap.Message
	for msg := range messages {
		result = append(result, msg)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch new messages: %v", err)
	}

	return result, nil
}

// func handleNewEmail(msg *imap.Message) {
// 	if msg.Envelope == nil {
// 		log.Println("Received email with no envelope")
// 		return
// 	}

// 	log.Printf("📧 New Email Received!")
// 	log.Printf("Subject: %s", msg.Envelope.Subject)

// 	if len(msg.Envelope.From) > 0 {
// 		from := msg.Envelope.From[0]
// 		if from.PersonalName != "" {
// 			log.Printf("From: %s <%s>", from.PersonalName, from.MailboxName+"@"+from.HostName)
// 		} else {
// 			log.Printf("From: %s@%s", from.MailboxName, from.HostName)
// 		}
// 	}

// 	if msg.Envelope.Date != nil {
// 		log.Printf("Date: %s", msg.Envelope.Date.Format("2006-01-02 15:04:05"))
// 	}

// }

func InitMonitor(callback func(*imap.Message)) {
	// mailboxes, err := ImapClient.ListMailboxes()
	// if err != nil {
	// 	log.Printf("Failed to list mailboxes: %v", err)
	// } else {
	// 	fmt.Println("Available mailboxes:")
	// 	for _, mbox := range mailboxes {
	// 		fmt.Printf("- %s\n", mbox.Name)
	// 	}
	// }

	// messages, err := ImapClient.FetchEmails("INBOX", 5)
	// if err != nil {
	// 	log.Printf("Failed to fetch emails: %v", err)
	// } else {
	// 	fmt.Printf("Found %d recent messages\n", len(messages))
	// 	for _, msg := range messages {
	// 		if msg.Envelope != nil {
	// 			fmt.Printf("Subject: %s\n", msg.Envelope.Subject)
	// 			if len(msg.Envelope.From) > 0 {
	// 				fmt.Printf("From: %s\n", msg.Envelope.From[0].PersonalName)
	// 			}
	// 			body, err := ImapClient.GetMessageBody(msg)
	// 			fmt.Printf("Body: %s\n", body, err)
	// 		}
	// 	}
	// }

	monitor := NewEmailMonitor(ImapClient, 30*time.Second, callback) // Every 30 seconds
	if err := monitor.Start(); err != nil {
		log.Fatalf("Failed to start email monitor: %v", err)
	}
}
