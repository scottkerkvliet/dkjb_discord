package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Configurable constants
const unsubscribeCommand = "!unsubscribe"
const factCommand = "!fact"
const frequencyCommand = "!frequency"
const imageCommand = "!image"
const autoCommand = "!auto"
const gifCommand = "!gif"
const factMinutes = 3
const factMinutesFast = 0.5
const pauseSeconds = 3

// Constants for execution
const pause = time.Second * pauseSeconds

// variables for execution
var channelId string
var ticker *time.Ticker
var factChannel chan bool
var priorityUsers []*discordgo.User
var fastMode = false
var auto = true

func main() {
	rand.Seed(time.Now().UnixNano())
	// Load Files
	openingMessageFile, err := ioutil.ReadFile("messages/opening-message.txt")
	if err != nil {
		fmt.Println("Could not read opening-message.txt")
		return
	}

	channelIdFile, err := ioutil.ReadFile("channelId.txt")
	if err != nil {
		fmt.Println("Could not read channelId.txt")
		return
	}
	channelId = string(channelIdFile)

	botTokenFile, err := ioutil.ReadFile("botToken.txt")
	if err != nil {
		fmt.Println("Could not read botToken.txt")
		return
	}

	// Create bot
	bot, err := discordgo.New("Bot " + string(botTokenFile))
	if err != nil {
		fmt.Println("Could not create discord session")
		return
	}
	bot.Identify.Intents = discordgo.IntentsGuildMessages

	err = bot.Open()
	if err != nil {
		fmt.Println("Error opening connection")
		return
	}
	defer bot.Close()

	// Initialize variables
	ticker = time.NewTicker(getDuration())
	factChannel = make(chan bool)

	// Initialize bot
	bot.AddHandler(messageCreate)
	openingMessage := fmt.Sprintf("%v\n", mentionEveryone())
	openingMessage += string(openingMessageFile)
	openingMessage += fmt.Sprintf("\nI am currently configured to deliver doses of trivia every %v.", getDurationText(factMinutes))
	bot.ChannelMessageSend(channelId, openingMessage)
	commandMessage := "List of commands:\n"
	commandMessage += fmt.Sprintf("`%v` to get a new fact right away!\n", factCommand)
	commandMessage += fmt.Sprintf("`%v` for a fun Donkey Kong image!\n", imageCommand)
	commandMessage += fmt.Sprintf("`%v` to change the frequency of automated fact delivery\n", frequencyCommand)
	commandMessage += fmt.Sprintf("`%v` if you no longer wish to receive DKJB facts\n", unsubscribeCommand)
	commandMessage += "Commands v2:\n"
	commandMessage += fmt.Sprintf("`%v` to toggle the automated fact delivery\n", autoCommand)
	commandMessage += fmt.Sprintf("`%v` to print out a gif\n", gifCommand)
	bot.ChannelMessageSend(channelId, commandMessage)

	go sendFacts(bot)
	triggerDelayedFact()
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	// Main loop
	waitUntilClose()
}

func waitUntilClose() {
	// Wait here until CTRL-C or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	for {
		select {
		case <-sc:
			return
		case <-ticker.C:
			factChannel <- true
		}
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, unsubscribeCommand) {
		priorityUsers = append(priorityUsers, m.Author)
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Thank you for joining our priority list. You are now a priority user, %v, and we will notify you when new facts are ready!", mentionUser(m.Author)))
		triggerDelayedFact()
	} else if strings.HasPrefix(m.Content, factCommand) {
		triggerFact()
	} else if strings.HasPrefix(m.Content, frequencyCommand) {
		if fastMode {
			return
		}
		fastMode = true
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Thank you for your enthusiasm! We have increased the frequency to a fact every %v!", getDurationText(factMinutesFast)))
		triggerDelayedFact()
	} else if strings.HasPrefix(m.Content, imageCommand) {
		sendImage(s, m)
	} else if strings.HasPrefix(m.Content, gifCommand) {
		sendGif(s, m)
	} else if strings.HasPrefix(m.Content, autoCommand) {
		auto = !auto
		if auto {
			s.ChannelMessageSend(m.ChannelID, "Automated fact delivery is enabled again!")
			triggerFact()
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("We've disabled automated fact delivery. Type `%v` to re-enable!", autoCommand))
			ticker.Stop()
		}
	}
}

func mentionUser(user *discordgo.User) string {
	return fmt.Sprintf("<@%v>", user.ID)
}

func mentionEveryone() string {
	return "@everyone"
}

func sendFacts(s *discordgo.Session) {
	for range factChannel {
		var facts []string
		factsFile, err := os.Open("messages/facts.txt")
		if err != nil {
			fmt.Println("Could not open facts.txt")
			return
		}
		scanner := bufio.NewScanner(factsFile)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			facts = append(facts, scanner.Text())
		}
		factsFile.Close()

		factIndex := rand.Intn(len(facts))

		newFact := facts[factIndex]

		if len(priorityUsers) > 0 {
			var userList string
			for _, user := range priorityUsers {
				userList += fmt.Sprintf("%v ", mentionUser(user))
			}
			newFact = userList + newFact
		}
		s.ChannelMessageSend(channelId, newFact)
	}
}

func sendImage(s *discordgo.Session, m *discordgo.MessageCreate) {
	imageFile, err := os.Open("images/image1.jpg")
	if err != nil {
		fmt.Println("Could not open image1.jpg")
		return
	}
	defer imageFile.Close()
	s.ChannelFileSend(m.ChannelID, "iwanturbigbanana.jpg", imageFile)
}

func sendGif(s *discordgo.Session, m *discordgo.MessageCreate) {
	gifDir, err := os.Open("gifs")
	if err != nil {
		fmt.Println("Could not open gif directory")
		return
	}
	defer gifDir.Close()

	gifList, err := gifDir.Readdirnames(0)
	if err != nil {
		fmt.Println("Could not open gif file names")
		return
	}

	gifIndex := rand.Intn(len(gifList))
	gifFile, err := os.Open(fmt.Sprintf("gifs/%v", gifList[gifIndex]))
	if err != nil {
		fmt.Println("Could not open gif")
		return
	}
	defer gifFile.Close()

	s.ChannelFileSend(m.ChannelID, gifList[gifIndex], gifFile)
}

func triggerFact() {
	if auto {
		ticker.Reset(getDuration())
	}
	factChannel <- true
}

func triggerDelayedFact() {
	if auto {
		ticker.Reset(getDuration())
	}
	time.AfterFunc(pause, func() {
		factChannel <- true
	})
}

func getDuration() time.Duration {
	minutes := float64(factMinutes)
	if fastMode {
		minutes = float64(factMinutesFast)
	}
	return time.Duration(minutes * float64(time.Minute))
}

func getDurationText(minutes float64) string {
	if minutes == 1 {
		return "minute"
	} else if minutes > 1 {
		roundedMinutes := math.Round(minutes)
		return fmt.Sprintf("%v minutes", roundedMinutes)
	} else {
		roundedSeconds := math.Round(minutes * 60)
		return fmt.Sprintf("%v seconds", roundedSeconds)
	}
}
