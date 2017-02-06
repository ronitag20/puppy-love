package service

import (
	"log"
	"net/smtp"
	"time"

	"github.com/pclubiitk/puppy-love/db"
	"github.com/pclubiitk/puppy-love/models"
	"github.com/pclubiitk/puppy-love/utils/config"

	"gopkg.in/mgo.v2/bson"
)

func QueueService(listen_channel chan string, signup_channel chan string) {
	for request := range listen_channel {
		go func(request string) {
			signup_channel <- request
		}(request)
	}
}

func SignupService(
	Db db.PuppyDb,
	signup_channel chan string,
	mail_channel chan models.User) {

	for id := range signup_channel {
		time.Sleep(3000 * time.Millisecond)
		log.Println("Signing up: " + id)

		u := models.User{}

		// If no such user
		if err := Db.GetById("user", id).One(&u); err != nil {
			log.Print(err)
			continue
		}

		// If user has already been computed
		if u.Dirty == false {
			log.Print("User ", id, " is not dirty. Skipping.")

			// Mailing should be async
			go func(user models.User) {
				mail_channel <- user
			}(u)

			continue
		}

		req_gender := "0"
		if u.Gender == "1" {
			req_gender = "0"
		} else {
			req_gender = "1"
		}

		// Compute table steps
		type typeIds struct {
			Id string `json:"_id" bson:"_id"`
		}

		var people []typeIds
		collection := Db.GetCollection("user")
		err := collection.Find(bson.M{"gender": req_gender, "dirty": false}).All(&people)

		if err != nil {
			log.Println("SignupService: Cannot fetch gender list: ", req_gender)
			log.Println(err)
			return
		}

		cnt := 0
		compute_coll := Db.GetCollection("compute")
		for _, fe := range people {
			log.Println("SignupService: "+fe.Id, "-", id, "-", cnt)
			cnt = cnt + 1
			res := models.UpsertEntry(fe.Id, id)
			compute_coll.Upsert(res.Selector, res.Change)
		}

		log.Println("SignupService: Finished signup for " + id)

		// Mark user as not dirty
		if _, err := Db.GetById("user", id).
			Apply(u.MarkNotDirty(), &u); err != nil {

			log.Println("SignupService: Could not mark ", id, " as not dirty")
			log.Println(err)
			return
		}

		// Mailing should be async
		go func(user models.User) {
			mail_channel <- user
		}(u)
	}
}

func MailerService(Db db.PuppyDb, mail_channel chan models.User) {

	for u := range mail_channel {
		auth := smtp.PlainAuth("", config.EmailUser, config.EmailPass,
			"smtp.cc.iitk.ac.in")
		to := []string{u.Email + "@iitk.ac.in"}
		msg := []byte("To: " + u.Email + "@iitk.ac.in" + "\r\n" +
			"Subject: Puppy-Love authentication code\r\n" +
			"\r\n" +
			"Use this token while signing up, and don't share it with anyone.\n" +
			"Token: " + u.AuthC + "\n" +
			".\r\n")
		smtp.SendMail("smtp.cc.iitk.ac.in:25", auth,
			config.EmailUser+"@iitk.ac.in", to, msg)

		log.Println("Mailed " + u.Id)
	}
}
