package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type ShareState struct {
	mutex          sync.Mutex
	nActors        int
	nMsgsProcessed int
	nMsgsSent      int
}

func (s *ShareState) incrementMsgsSent() {
	s.mutex.Lock()
	s.nMsgsSent += 1
	s.mutex.Unlock()
}

func (s *ShareState) incrementMsgsProcessed() {
	s.mutex.Lock()
	s.nMsgsProcessed += 1
	s.mutex.Unlock()
}

type Message struct {
	msg    interface{}
	sender string
}

type Actor struct {
	id        string
	channels  *map[string]chan Message
	shareData *ShareState
	// children  []*Actor
	// parent	*Actor
}

func new(id string, channels *map[string]chan Message, channelCap int, sharedData *ShareState) (*Actor, error) {
	if id == "all" {
		return nil, errors.New("Actor id 'all' is reserved")
	}
	if _, ok := (*channels)[id]; ok {
		return nil, errors.New("Actor id already exists")
	}
	channelsDeref := *channels
	channelsDeref[id] = make(chan Message, channelCap)
	fmt.Printf("Actor %s - Created\n", id)
	sharedData.mutex.Lock()
	sharedData.nActors += 1
	sharedData.mutex.Unlock()
	new_actor := Actor{id, channels, sharedData}

	return &new_actor, nil
}

func (a *Actor) send(m string, to string) error {
	channels := *a.channels
	if len(channels) <= 1 {
		return errors.New("No other actors to send message to")
	}

	if to == "all" {
		for k, _ := range channels {
			if k != a.id {
				select {
				case channels[k] <- Message{m, a.id}:
					fmt.Printf("Actor %s - Sending message '%s' to 'all' - %s\n", a.id, m, k)
					a.shareData.incrementMsgsSent()
				default:
					fmt.Printf("Actor %s - Channel is full\n", a.id)
					return errors.New("Channel is full")
				}
			}
		}
	} else {
		if _, ok := channels[to]; !ok {
			errors.New("Channel not found!")
		}
		// prevents blocking the sender
		select {
		case channels[to] <- Message{m, a.id}:
			fmt.Printf("Actor %s - Sending message '%s' to %s\n", a.id, m, to)
			a.shareData.incrementMsgsSent()
		default:
			fmt.Printf("Actor %s - Channel is full\n", a.id)
			return errors.New("Channel is full")
		}
	}
	return nil
}

func (a *Actor) receiveOnce() (Message, error) {
	channels := *a.channels
	select {
	case msg := <-channels[a.id]:
		a.shareData.incrementMsgsProcessed()
		fmt.Printf("Actor %s - Processed message '%s' from sender %s\n", a.id, msg.msg, msg.sender)
		return msg, nil
	default:
		fmt.Printf("Actor %s - No messages to process.\n", a.id)
		return Message{}, nil
	}
}

func (a *Actor) receiveMany() ([]*Message, error) {
	channels := *a.channels
	var messages []*Message
	// If the bellow snip was used instead, it would wait until channel was closed
	// for msg := range channels[a.id]
	for {
		select {
		case msg := <-channels[a.id]:
			messages = append(messages, &msg)
			a.shareData.incrementMsgsProcessed()
			fmt.Printf("Actor %s - Processed message '%s' from sender %s\n", a.id, msg.msg, msg.sender)
		default:
			return messages, nil
		}
	}
}

func start(actors []*Actor) {
	fmt.Println("Starting actors...")
	for _, actor := range actors {
		// synchronous call, so it waits for the actor to finish processing messages
		// alternative would be to use a wait group (TODO)
		func(a Actor) {
			if _, err := a.receiveMany(); err != nil {
				fmt.Printf("Actor %s - No messages to process.\n", a.id)
			}
		}(*actor)
	}
}

// TODO: child actors; add functions to execute
// TODO: ActorManager
func main() {
	messageChannel := make(map[string]chan Message)
	// sending more messages to a channel then its capacity will block the sender
	// until messages are processed, and thus releasing space in the buffer.
	// Since the senders are goroutines, the program is likely to terminate before
	// the sender is able to complete, resulting in wrong output of sharedData``.
	// In this example, a channel receives a maximum of 3 messages,
	// so capacity of 3 is enough to avoid blocking the sender; using `select` too, since
	// if the buffer is full it will execute the `default` block.
	// Setting the right buffer size guarantees that all messages are processed and sent.
	// Try changing the buffer size to 1 and see what happens.
	channelCap := 3
	sharedData := ShareState{nActors: 0, nMsgsProcessed: 0, nMsgsSent: 0}
	a1, _ := new("1", &messageChannel, channelCap, &sharedData)
	a2, _ := new("2", &messageChannel, channelCap, &sharedData)
	a3, _ := new("3", &messageChannel, channelCap, &sharedData)
	fmt.Printf("N Actors: %d | Messages Sent: %d | Messages Received: %d\n", sharedData.nActors, sharedData.nMsgsSent, sharedData.nMsgsProcessed)
	fmt.Println("---")

	// send messages
	// using sleep makes it easier to see the messages being processed
	go a1.send("Viva 2", "2")
	time.Sleep(1 * time.Second)
	go a1.send("Bonjour 3", "3")
	time.Sleep(1 * time.Second)
	go a2.send("Olá 1", "1")
	time.Sleep(1 * time.Second)
	go a3.send("Hello World", "all")
	time.Sleep(1 * time.Second)
	go a3.send("Olá Mundo!", "all")
	time.Sleep(1 * time.Second)
	fmt.Println("---")

	// await
	start([]*Actor{a1, a2, a3})

	fmt.Println("---")
	fmt.Printf("N Actors: %d | Messages Sent: %d | Messages Received: %d\n", sharedData.nActors, sharedData.nMsgsSent, sharedData.nMsgsProcessed)
}
