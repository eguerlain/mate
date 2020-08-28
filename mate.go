package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

const TIME_FORMAT = "2006/01/02 15:04:05"
const STOP_TOKEN = "mate:STOP"
const DB_NAME = ".mate.csv"
const CSV_HEADER = "timestamp,title\n"
const WORK_DAY = time.Hour*7 + time.Minute*30

type Record struct {
	timestamp time.Time
	title     string
}

func getDbPath() string {
	// return "./mate.csv"
	homePath := os.Getenv("HOME")
	if homePath == "" {
		log.Fatal("Cannot access home directory")
	}
	var dbPath strings.Builder
	dbPath.WriteString(homePath)
	dbPath.WriteString("/")
	dbPath.WriteString(DB_NAME)

	return dbPath.String()
}

func ensureCSVExists() {
	f, err := os.OpenFile(getDbPath(), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	_, err = r.Read()
	if err != nil {
		if err == io.EOF {
			if _, err = f.WriteString(CSV_HEADER); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

func getRecords() (records []Record) {
	ensureCSVExists()
	f, err := os.OpenFile(getDbPath(), os.O_RDONLY, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)

	rawRecords, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	for index, rawRecord := range rawRecords {
		if index == 0 {
			continue
		}

		timestamp, err := time.Parse(TIME_FORMAT, rawRecord[0])
		if err != nil {
			log.Fatal(err)
		}

		record := Record{
			timestamp,
			rawRecord[1],
		}

		records = append(records, record)
	}

	return
}

// Writes a new entry to the CSV
func writeTicket(title string) {
	ensureCSVExists()
	now := time.Now().Format(TIME_FORMAT)

	var literalRecord strings.Builder

	literalRecord.WriteString("\"")
	literalRecord.WriteString(now)
	literalRecord.WriteString("\"")
	literalRecord.WriteString(",")
	literalRecord.WriteString("\"")
	literalRecord.WriteString(title)
	literalRecord.WriteString("\"")
	literalRecord.WriteString("\n")

	f, err := os.OpenFile(getDbPath(), os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if _, err = f.WriteString(literalRecord.String()); err != nil {
		log.Fatal(err)
	}
}

func startTicket(title string) {
	writeTicket(title)
	fmt.Printf("STARTING %s\n", title)
}

func stopTicket() {
	records := getRecords()

	working := false // To check if the file is not empty or that the previous entry is not already a STOP
	var last Record
	if len(records) != 0 {
		last = records[len(records)-1]
		if last.title != STOP_TOKEN {
			working = true
		}
	}

	if working {
		writeTicket(STOP_TOKEN)
		fmt.Printf("STOPPING %s\n", last.title)
	} else {
		fmt.Println("Not currently working on a ticket. Run:\n$ mate start [\"Ticket title\"]")
	}
}

func yellForNoRecord() {
	fmt.Println("No entry saved for now. Run:\n$ mate start \"Ticket title\"")
	os.Exit(1)
}

func yellForNoPreviousTicket() {
	fmt.Println("Can not find a previous ticket to restart. Run:")
	fmt.Println("$ mate start \"Ticket title\"")
	os.Exit(1)
}

func yellForNotStopped(currentTicketTitle string) {
	fmt.Printf("You are currently working on: %s\n", currentTicketTitle)
	os.Exit(1)
}

func restartLastTicket() {
	records := getRecords()
	numberOfRecords := len(records)

	switch {
	case numberOfRecords == 0:
		yellForNoRecord()
	case numberOfRecords == 1:
		if records[0].title == STOP_TOKEN {
			yellForNoPreviousTicket()
		} else {
			yellForNotStopped(records[0].title)
		}
	case numberOfRecords > 1:
		last := records[len(records)-1]
		if last.title == STOP_TOKEN {
			penultimate := records[len(records)-2]
			if penultimate.title == STOP_TOKEN {
				yellForNoPreviousTicket()
			} else {
				startTicket(penultimate.title)
			}
		} else {
			yellForNotStopped(last.title)
		}
	}
}

// Computes the duration of each entry (including STOP entries, but not if last)
// Keeps tickets in the order of entries
func computeEntriesDuration(records []Record) (tickets []struct {
	title    string
	duration time.Duration
}) {
	if len(records) == 0 {
		return
	}

	var (
		currentTicketTitle string
		startTime          time.Time
		ticket             struct {
			title    string
			duration time.Duration
		}
	)

	for i, r := range records {
		if i != 0 {
			ticket.title = currentTicketTitle
			ticket.duration = r.timestamp.Sub(startTime)
			tickets = append(tickets, ticket)
		}
		currentTicketTitle, startTime = r.title, r.timestamp
	}
	// Compute the duration of the last ticket, if not a STOP
	last := records[len(records)-1]
	if last.title != STOP_TOKEN {
		ticket.title = last.title
		now, _ := time.Parse(TIME_FORMAT, (time.Now().Format(TIME_FORMAT)))
		ticket.duration = now.Sub(startTime)
		tickets = append(tickets, ticket)
	}
	return
}

// Removes the STOP tickets
// Keeps the order of tickets
func filterStops(tickets []struct {
	title    string
	duration time.Duration
}) (outTickets []struct {
	title    string
	duration time.Duration
}) {
	for _, t := range tickets {
		if t.title != STOP_TOKEN {
			outTickets = append(outTickets, t)
		}
	}
	return
}

// Computes the durations per ticket (grouping several entries)
// Returns a map (loosing the order of entries)
func groupDurations(tickets []struct {
	title    string
	duration time.Duration
}) (groupedTickets map[string]time.Duration) {
	groupedTickets = make(map[string]time.Duration)
	for _, t := range tickets {
		groupedTickets[t.title] += t.duration
	}
	return
}

// Computes the total time worked since the begining of the database
func computeTotalTime(tickets []struct {
	title    string
	duration time.Duration
}) (totalTime time.Duration) {
	for _, t := range tickets {
		totalTime += t.duration
	}
	return
}

func listEntries() {
	tickets := computeEntriesDuration(getRecords())

	if len(tickets) == 0 {
		fmt.Println("Nothing to show (yet)")
		return
	}

	for _, t := range tickets {
		if t.title == STOP_TOKEN {
			fmt.Printf("---\n")
		} else {
			fmt.Printf("%s\t%v\n", t.title, t.duration)
		}
	}
}

func showReport() {
	tickets := groupDurations(filterStops(computeEntriesDuration(getRecords())))

	if len(tickets) == 0 {
		fmt.Println("Nothing to show (yet)")
		return
	}

	for key, value := range tickets {
		fmt.Printf("%s\t%v\n", key, value)
	}
}

// Return the title of the last ticket
// A STOP_TOKEN is returned if no record in database
func getLastTicketTitle() (status string) {
	records := getRecords()

	if len(records) == 0 {
		status = STOP_TOKEN
	} else {
		status = records[len(records)-1].title
	}

	return
}

func showInfo() {
	tickets := filterStops(computeEntriesDuration(getRecords()))
	totalTime := computeTotalTime(tickets)
	dayDiff := WORK_DAY - totalTime
	status := getLastTicketTitle()

	if status == STOP_TOKEN {
		fmt.Printf("Currently not working\n")
	} else {
		groupedTickets := groupDurations(tickets)
		fmt.Printf("Working on %s (%v)\n", status, groupedTickets[status])
	}

	if dayDiff > 0 {
		fmt.Printf("Still %v to work\n", dayDiff)
	} else {
		fmt.Printf("You're done for today (+%v)\n", dayDiff*-1)
	}
	// Currently [not working] / [working on #XXXX (xxmxxs)]
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func clearEntries() {
	reader := bufio.NewReader(os.Stdin)
	var userEntry string
	i := 0
	for !contains([]string{"y\n", "Y\n", "n\n", "N\n", "\n"}, userEntry) && i < 3 {
		fmt.Print("Empty all entries in the database? [y/N]: ")
		userEntry, _ = reader.ReadString('\n')
		i++
	}
	switch userEntry {
	case "y\n", "Y\n":
		err := os.Truncate(getDbPath(), int64(len(CSV_HEADER)))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Database cleared")
	default:
		fmt.Println("Command canceled")
	}
}

func showErrorHelp() {
	fmt.Println("Please provide a command among:")
	fmt.Println("  * start (s)")
	fmt.Println("  * stop (x)")
	fmt.Println("  * log (l)")
	fmt.Println("  * list (ll)")
	fmt.Println("  * info (i)")
	fmt.Println("  * clear")
}

func main() {
	numberOfArgs := len(os.Args)

	if numberOfArgs == 1 {
		showErrorHelp()
		os.Exit(1)
	}

	if numberOfArgs > 3 {
		fmt.Println("Too much arguments provided.")
		fmt.Println("(Use quotes for long titles)")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "start", "s":
		if numberOfArgs == 3 {
			startTicket(os.Args[2])
		} else {
			restartLastTicket()
		}
	case "stop", "x":
		if numberOfArgs == 3 {
			fmt.Println("The stop command does not take any parameter")
			os.Exit(1)
		}
		stopTicket()
	case "log", "l":
		if numberOfArgs == 3 {
			fmt.Println("The log command does not take any parameter")
			os.Exit(1)
		}
		showReport()
	case "list", "ll":
		if numberOfArgs == 3 {
			fmt.Println("The list command does not take any parameter")
			os.Exit(1)
		}
		listEntries()
	case "info", "i":
		if numberOfArgs == 3 {
			fmt.Println("The info command does not take any parameter")
			os.Exit(1)
		}
		showInfo()
	case "clear":
		if numberOfArgs == 3 {
			fmt.Println("The clear command does not take any parameter")
			os.Exit(1)
		}
		clearEntries()
	default:
		showErrorHelp()
	}
}
