package main

import (
	"bufio"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	. "image"

	"github.com/Pallinder/go-randomdata"
	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
	commonDAO "github.com/toasterlint/DAWS/common/dao"
	commonModels "github.com/toasterlint/DAWS/common/models"
	. "github.com/toasterlint/DAWS/common/utils"
	worldDAO "github.com/toasterlint/DAWS/world_controller/dao"
	"gopkg.in/mgo.v2/bson"
)

var conn *amqp.Connection
var ch *amqp.Channel
var worldq, worldtrafficq, worldcityq, cityjobq, trafficjobq amqp.Queue
var msgs <-chan amqp.Delivery
var runTrigger bool
var controllers []commonModels.Controller
var settings commonModels.Settings
var dao = worldDAO.DAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}
var commondao = commonDAO.DAO{Server: "mongo.daws.xyz", Database: "daws", Username: "daws", Password: "daws"}
var numCities = 0
var numBuildings = 0
var numPeople = 0
var looper = 0

func startHTTPServer() {
	r := mux.NewRouter()
	r.Handle("/", http.FileServer(http.Dir("./html")))
	r.HandleFunc("/api/status", apiStatus).Methods("GET")
	r.HandleFunc("/api/triggerNext", apiTrigger).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func connectQueues() {
	var err error
	conn, err = amqp.Dial("amqp://guest:guest@rabbitmq.daws.xyz:5672")
	FailOnError(err, "Failed to connect to RabbitMQ")

	ch, err = conn.Channel()
	FailOnError(err, "Failed to open a channel")

	worldq, err = ch.QueueDeclare(
		"world_queue", //name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	FailOnError(err, "Failed to declare queue")

	worldtrafficq, err = ch.QueueDeclare(
		"world_traffic_queue", //name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	FailOnError(err, "Failed to declare queue")

	Logger.Printf("World Traffic Queue Consumers: %d", worldtrafficq.Consumers)

	worldcityq, err = ch.QueueDeclare(
		"world_city_queue", //name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	FailOnError(err, "Failed to declare queue")

	Logger.Printf("World City Queue Consumers: %d", worldcityq.Consumers)

	cityjobq, err = ch.QueueDeclare(
		"city_job_queue", // name
		true,             // durable
		false,            //delete when unused
		false,            // exclusive
		false,            // no wait
		nil,              // arguments
	)
	FailOnError(err, "Failed to declare City Job Queue")

	trafficjobq, err = ch.QueueDeclare(
		"traffic_job_queue", // name
		true,                // durable
		false,               //delete when unused
		false,               // exclusive
		false,               // no wait
		nil,                 // arguments
	)
	FailOnError(err, "Failed to declare Traffic Job Queue")

	err = ch.Qos(
		1,     //prefetch count
		0,     //prefetch size
		false, //global
	)

	msgs, err = ch.Consume(
		worldq.Name, //queue
		"",          //consumer
		false,       //auto-ack
		false,       //exclusive
		false,       //no-local
		false,       //no-wait
		nil,         //args
	)
	FailOnError(err, "Failed to register a consumer")
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	LogToConsole("API Call made: status")
	w.Write([]byte("API Call made: status"))
}

func apiTrigger(w http.ResponseWriter, r *http.Request) {
	cityids, err := commondao.GetAllCityIDs()
	FailOnError(err, "Failed to get city IDs")
	msg := &commonModels.WorldTrafficQueueMessage{WorldSettings: settings}
	triggerNext(cityids, msg)
	LogToConsole("Manually Trigger")
	w.Write([]byte("Manually triggered"))
}

func triggerNext(cities []commonDAO.Mongoid, worldtrafficmessage *commonModels.WorldTrafficQueueMessage) {
	tempMsgJSON, _ := json.Marshal(worldtrafficmessage)
	err := ch.Publish(
		"",                 // exchange
		worldtrafficq.Name, // routing key
		false,              // mandatory
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         []byte(tempMsgJSON),
		})
	FailOnError(err, "Failed to post to World Traffic Queue")
	for _, element := range cities {
		tempMsg := &commonModels.WorldCityQueueMessage{WorldSettings: settings, City: element.ID.Hex()}
		tempMsgJSON, _ := json.Marshal(tempMsg)
		err := ch.Publish(
			"",              // exchange
			worldcityq.Name, // routing key
			false,           // mandatory
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "application/json",
				Body:         []byte(tempMsgJSON),
			})
		FailOnError(err, "Failed to post to World City Queue")
	}
}

func processTrigger() {
	realLastTime := time.Now()
	go printStatus()
	for runTrigger {
		time.Sleep(time.Microsecond * 500)
		// first check if all controllers are ready (and that we have any)
		if len(controllers) == 0 {
			LogToConsole("No controllers")
			time.Sleep(time.Second * 5)
			continue
		}
		readyt := true
		readyc := true
		totalt := 0
		totalc := 0
		for i := range controllers {
			if controllers[i].Type == "traffic" {
				totalt++
			} else {
				totalc++
			}
			if controllers[i].Ready == false {
				if controllers[i].Type == "traffic" {
					readyt = false
				} else {
					readyc = false
				}
				break
			}
		}
		if readyt == false || readyc == false {
			continue
		}
		if totalt == 0 || totalc == 0 {
			continue
		}
		// make sure we don't go over max speed limit
		t := time.Now()
		dur := t.Sub(realLastTime)
		for i := range controllers {
			controllers[i].Ready = false
		}
		if dur > time.Duration(settings.WorldSpeed)*time.Millisecond {
			LogToConsole("Warning: world processing too slow, last duration was - " + dur.String())
		}
		cityids, err := commondao.GetAllCityIDs()
		FailOnError(err, "Failed to get city IDs")
		msg := &commonModels.WorldTrafficQueueMessage{WorldSettings: settings}
		triggerNext(cityids, msg)
		settings.LastTime = settings.LastTime.Add(time.Second * 1)
		realLastTime = time.Now()
	}
}

func processMsgs() {
	for d := range msgs {
		tempController := commonModels.Controller{}
		json.Unmarshal(d.Body, &tempController)
		found := false
		var tempRemove int
		for i := range controllers {
			if controllers[i].ID == tempController.ID {
				found = true
				controllers[i].Ready = tempController.Ready
				tempRemove = i
				break
			}
		}
		//LogToConsole("Controller found stats: " + strconv.FormatBool(found))
		if found == false {
			controllers = append(controllers, tempController)
		}
		// Remove the controller if it sent exit true
		if tempController.Exit == true {
			controllers = append(controllers[:tempRemove], controllers[tempRemove+1:]...)
		}
		//LogToConsole("Done")
		d.Ack(false)
	}
}

func runConsole() {
	// setup terminal
	reader := bufio.NewReader(os.Stdin)
ReadCommand:
	Logger.Print("Command: ")
	text, _ := reader.ReadString('\n')
	text = strings.Trim(text, "\n")
	switch text {
	case "exit":
		Logger.Print("Purging queues")
		_, err := ch.QueuePurge(worldcityq.Name, false)
		FailOnError(err, "Failed to purge World City Queue")
		_, err = ch.QueuePurge(worldtrafficq.Name, false)
		FailOnError(err, "Failed to purge World Traffic Queue")
		_, err = ch.QueuePurge(worldq.Name, false)
		FailOnError(err, "Failed to purge World Queue")
		Logger.Println("Saving settings...")
		err = dao.SaveSettings(settings)
		FailOnError(err, "Failed to save settings")
		Logger.Println("Exiting...")
		os.Exit(0)
	case "status":
		if runTrigger {
			Logger.Println("Running...")
		} else {
			Logger.Println("Stopped...")
		}

		tcontrollers := 0
		ccontrollers := 0
		for i := range controllers {
			if controllers[i].Type == "traffic" {
				tcontrollers++
			} else {
				ccontrollers++
			}
		}
		Logger.Printf("Traffic Controllers: %d", tcontrollers)
		Logger.Printf("City Controllers: %d", ccontrollers)
		Logger.Printf("Current Real Time: %s", time.Now().Format("2006-01-02 15:04:05"))
		Logger.Printf("Current Simulated Time: %s", settings.LastTime.Format("2006-01-02 15:04:05"))
	case "start":
		runTrigger = true
		go processTrigger()
		Logger.Println("World simulation started")
	case "stop":
		runTrigger = false
		Logger.Println("World simulation stopped")
	case "help":
		fallthrough
	default:
		Logger.Println("Help: ")
		Logger.Println("   status - Check the status of the world")
		Logger.Println("   start - Start world simulation")
		Logger.Println("   stop - Stop world simulation")
		Logger.Println("   exit - Exit the App")
	}
	goto ReadCommand
}

func loadConfig() {
	dao.Connect()
	commondao.Connect()
	var err error
	settings, err = dao.LoadSettings()
	FailOnError(err, "Failed to load settings")
	if settings.ID.Valid() {
		sett, _ := json.Marshal(settings)
		Logger.Println(string(sett))
	} else {
		LogToConsole("No settings found, creating defaults")
		var tempSettings commonModels.Settings
		tempSettings.CarAccidentFatalityRate = 0.0001159
		tempSettings.ID = bson.NewObjectId()
		tempSettings.LastTime = time.Now()
		tempSettings.MurderRate = 0.000053
		tempSettings.ViolentCrimeRate = 0.00381
		tempSettings.WorldSpeed = 5000
		var speeds = []commonModels.SpeedLimit{}
		var citySpeed = commonModels.SpeedLimit{Location: "city", Value: 35}
		var noncitySpeed = commonModels.SpeedLimit{Location: "noncity", Value: 70}
		speeds = append(speeds, citySpeed)
		speeds = append(speeds, noncitySpeed)
		tempSettings.SpeedLimits = speeds
		tempSettings.Diseases = []commonModels.Disease{}
		err := dao.InsertSettings(tempSettings)
		settings = tempSettings
		FailOnError(err, "Failed to insert settings")
	}
	getBuildingsCount()
	getCitiesCount()
	getPeopleCount()
}

func printStatus() {
	for runTrigger {
		if runTrigger {
			Logger.Println("Running...")
		} else {
			Logger.Println("Stopped...")
		}

		tcontrollers := 0
		ccontrollers := 0
		for i := range controllers {
			if controllers[i].Type == "traffic" {
				tcontrollers++
			} else {
				ccontrollers++
			}
		}
		Logger.Printf("Traffic Controllers: %d", tcontrollers)
		Logger.Printf("City Controllers: %d", ccontrollers)
		Logger.Printf("Current Real Time: %s", time.Now().Format("2006-01-02 15:04:05"))
		Logger.Printf("Current Simulated Time: %s", settings.LastTime.Format("2006-01-02 15:04:05"))
		time.Sleep(time.Second * 10)
	}
}

func main() {
	// Set some initial variables
	InitLogger()
	loadConfig()
	controllers = []commonModels.Controller{}

	//init rabbit
	connectQueues()
	defer conn.Close()
	defer ch.Close()
	go processMsgs()

	// Start Web Server
	go startHTTPServer()

	// Start Console
	go runConsole()

	// Check to see if fresh world
	if numBuildings == 0 || numCities == 0 || numPeople == 0 {
		aWholeNewWorld()
	}

	// start world simulation
	runTrigger = true
	go processTrigger()

	Logger.Println("done initializing")

	// Loop main thread
	forever := make(chan bool)
	<-forever
}

func getCitiesCount() {
	numCities, _ = commondao.GetCitiesCount()
	Logger.Printf("Number of Cities: %d", numCities)
}

func getPeopleCount() {
	numPeople, _ = commondao.GetPeopleCount()
	Logger.Printf("Nummber of People in world: %d", numPeople)
}

func getBuildingsCount() {
	numBuildings, _ = commondao.GetBuildingsCount()
	Logger.Printf("Nummber of Buildings in world: %d", numBuildings)
}

func aWholeNewWorld() {
	LogToConsole("Starting a whole new world... don't you dare close your eyes!")
	// Create a new city somewhere random in the world that is 1 sq mile
	newCity := commonModels.City{}
	newCity.ID = bson.NewObjectId()

	cityNameGenURL := "https://www.mithrilandmages.com/utilities/CityNamesServer.php?count=1&dataset=united_states&_=1531715721885"
	req, err := http.NewRequest("GET", cityNameGenURL, nil)
	FailOnError(err, "Failed on http.NewRequest")
	webClient := &http.Client{}
	resp, err := webClient.Do(req)
	FailOnError(err, "Failed to get name")
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err2 := ioutil.ReadAll(resp.Body)
		FailOnError(err2, "Failed to read html body")
		newCity.Name = string(bodyBytes)
	}
	newCity.TopLeft = Point{X: randomdata.Number(5274720), Y: randomdata.Number(5274720)}
	newCity.BottomRight = Point{X: newCity.TopLeft.X + 5280, Y: newCity.TopLeft.Y + 5280}
	newCity.Established = settings.LastTime
	err = commondao.CreateCity(newCity)
	FailOnError(err, "Failed to create new city")
	Logger.Printf("Created City: %s", newCity.Name)

	canWeFixIt(newCity)

}

func canWeFixIt(city commonModels.City) {
	LogToConsole("Yes we can!")
	newBuilding := commonModels.Building{}
	newBuilding.ID = bson.NewObjectId()
	newBuilding.BuildDate = settings.LastTime
	newBuilding.Floors = 1
	newBuilding.MaxOccupancy = 20
	newBuilding.Name = "Home"
	newBuilding.Type = commonModels.House
	newBuilding.TopLeft = Point{X: randomdata.Number(city.TopLeft.X, city.BottomRight.X-208), Y: randomdata.Number(city.TopLeft.Y, city.BottomRight.Y-208)}
	newBuilding.BottomRight = Point{X: newBuilding.TopLeft.X + 208, Y: newBuilding.TopLeft.Y + 208}
	newBuilding.CityID = city.ID
	err := commondao.CreateBuilding(newBuilding)
	FailOnError(err, "Failed to create building")
	LogToConsole("Created a new home, Updating City")
	err = commondao.UpdateCity(city)
	FailOnError(err, "Failed to update City")
	justTheTwoOfUs(newBuilding)
}

func justTheTwoOfUs(building commonModels.Building) {
	LogToConsole("You and I")
	male := commonModels.Person{}
	female := commonModels.Person{}
	boyNameGenURL := "http://names.drycodes.com/1?nameOptions=boy_names"
	girlNameGenURL := "http://names.drycodes.com/1?nameOptions=girl_names"
	reqBoy, err := http.NewRequest("GET", boyNameGenURL, nil)
	FailOnError(err, "Error with Boy Name URL")
	reqGirl, err := http.NewRequest("GET", girlNameGenURL, nil)
	FailOnError(err, "Error with Girl Name URL")
	webClient := &http.Client{}
	respBoy, err := webClient.Do(reqBoy)
	FailOnError(err, "Error with Boy Name Request")
	respGirl, err := webClient.Do(reqGirl)
	FailOnError(err, "Error with Girl Name Request")
	defer respBoy.Body.Close()
	defer respGirl.Body.Close()
	if respBoy.StatusCode == http.StatusOK && respGirl.StatusCode == http.StatusOK {
		boybodyBytes, err := ioutil.ReadAll(respBoy.Body)
		FailOnError(err, "Failed to read body")
		mname := string(boybodyBytes)
		mname = strings.TrimPrefix(mname, "[\"")
		mname = strings.TrimSuffix(mname, "\"]")
		male.FirstName = strings.Split(mname, "_")[0]
		male.LastName = strings.Split(mname, "_")[1]
		girlbodyBytes, err := ioutil.ReadAll(respGirl.Body)
		FailOnError(err, "Failed to read body")
		fname := string(girlbodyBytes)
		fname = strings.TrimPrefix(fname, "[\"")
		fname = strings.TrimSuffix(fname, "\"]")
		female.FirstName = strings.Split(fname, "_")[0]
		female.LastName = male.LastName
	}
	male.Birthdate = settings.LastTime.AddDate(-18, 0, 0)
	female.Birthdate = male.Birthdate
	male.ChildrenIDs = []bson.ObjectId{}
	female.ChildrenIDs = male.ChildrenIDs
	male.CurrentBuilding = building.ID
	female.CurrentBuilding = male.CurrentBuilding
	male.CurrentXY = building.TopLeft
	female.CurrentXY = male.CurrentXY
	male.Happiness = 100
	female.Happiness = 100
	male.Health = 100
	female.Health = 100
	male.HomeBuilding = male.CurrentBuilding
	female.HomeBuilding = male.HomeBuilding
	male.ID = bson.NewObjectId()
	female.ID = bson.NewObjectId()
	male.NewToBuilding = false
	female.NewToBuilding = false
	male.Traveling = false
	female.Traveling = false
	male.WorkBuilding = male.HomeBuilding
	female.WorkBuilding = female.HomeBuilding
	male.Spouse = female.ID
	female.Spouse = male.ID
	errM := commondao.CreatePerson(male)
	FailOnError(errM, "Failed to create male")
	errF := commondao.CreatePerson(female)
	FailOnError(errF, "Failed to create female")

	Logger.Printf("People Names: %s %s, %s %s", male.FirstName, male.LastName, female.FirstName, female.LastName)
}
