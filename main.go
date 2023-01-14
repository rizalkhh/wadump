package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"time"

	"watgbridge/database"
	"watgbridge/state"
	"watgbridge/telegram"
	"watgbridge/utils"
	"watgbridge/whatsapp"

	"github.com/go-co-op/gocron"
)

func main() {
	// Load configuration file
	cfg := state.State.Config
	if len(os.Args) > 1 {
		cfg.Path = os.Args[1]
	} else {
		cfg.Path = "config.yaml"
	}
	err := cfg.LoadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	// Create local location for time
	if cfg.TimeZone == "" {
		cfg.TimeZone = "UTC"
	}
	locLoc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		log.Fatalln("could not set timezone to '" + cfg.TimeZone + "': " + err.Error())
	}
	state.State.LocalLocation = locLoc

	if cfg.WhatsApp.SessionName == "" {
		cfg.WhatsApp.SessionName = "watgbridge"
	}

	if cfg.GitExecutable == "" || cfg.GoExecutable == "" {
		gitPath, err := exec.LookPath("git")
		if err != nil && !errors.Is(err, exec.ErrDot) {
			log.Fatalln("failed to find path to git executable : " + err.Error())
		}

		goPath, err := exec.LookPath("go")
		if err != nil && !errors.Is(err, exec.ErrDot) {
			log.Fatalln("failed to find path to go executable : " + err.Error())
		}

		cfg.GitExecutable = gitPath
		cfg.GoExecutable = goPath

		log.Printf("Using '%s' and '%s' as path to executables for git and go\n",
			gitPath, goPath)

		cfg.SaveConfig()
	}

	// Setup database
	db, err := database.Connect()
	if err != nil {
		log.Fatalln("could not connect to database : " + err.Error())
	}
	state.State.Database = db
	err = database.AutoMigrate()
	if err != nil {
		log.Fatalln("could not migrate database tables : " + err.Error())
	}

	err = telegram.NewTelegramClient()
	if err != nil {
		panic(err)
	}
	log.Printf("[telegram] logged in as : %s [ @%s ]\n",
		state.State.TelegramBot.FirstName, state.State.TelegramBot.Username)
	telegram.AddTelegramHandlers()
	utils.TgRegisterBotCommands(state.State.TelegramBot, state.State.TelegramCommands...)

	err = whatsapp.NewWhatsAppClient()
	if err != nil {
		panic(err)
	}
	log.Printf("[whatsapp] logged in as : %s [ @%s ]\n",
		state.State.WhatsAppClient.Store.PushName, state.State.WhatsAppClient.Store.ID.User)
	state.State.WhatsAppClient.AddEventHandler(whatsapp.WhatsAppEventHandler)

	state.State.StartTime = time.Now().UTC()

	s := gocron.NewScheduler(time.UTC)
	s.TagsUnique()
	_, _ = s.Every(1).Hour().Tag("foo").Do(func() {
		contacts, err := state.State.WhatsAppClient.Store.Contacts.GetAllContacts()
		if err == nil {
			_ = database.ContactNameBulkAddOrUpdate(contacts)
		}
	})

	state.State.TelegramUpdater.Idle()
}
