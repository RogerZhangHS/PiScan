// Copyright Banrai LLC. All rights reserved. Use of this source code is
// governed by the license that can be found in the LICENSE file.

// This code initializes and runs the WebApp on the Pi client

package main

import (
	"flag"
	"fmt"
	"github.com/Banrai/PiScan/client/database"
	"github.com/Banrai/PiScan/client/ui"
	"log"
	"net/http"
	"path"
)

const (
	// server constants (runs on Pi client)
	SERVER_HOST = "localhost"
	SERVER_PORT = 8080

	// non-html mime types (ajax replies)
	MIME_JSON = "application/json"
)

func main() {
	var (
		host, apiHost, templatesFolder, dbPath, dbFile string
		port, apiPort                                  int
	)
	flag.StringVar(&host, "host", SERVER_HOST, fmt.Sprintf("Host name or IP address for this server (defaults to '%s')", SERVER_HOST))
	flag.IntVar(&port, "port", SERVER_PORT, fmt.Sprintf("Port addess for this server (defaults to '%d')", SERVER_PORT))
	flag.StringVar(&templatesFolder, "templates", "", "Path to the html templates (REQUIRED)")
	flag.StringVar(&dbPath, "dbPath", database.SQLITE_PATH, fmt.Sprintf("Path to the sqlite file (defaults to '%s')", database.SQLITE_PATH))
	flag.StringVar(&dbFile, "dbFile", database.SQLITE_FILE, fmt.Sprintf("The sqlite database file (defaults to '%s')", database.SQLITE_FILE))
	flag.Parse()

	// make sure the required parameters are passed when run
	if templatesFolder == "" || apiHost == "" {
		fmt.Println("WebApp usage:")
		flag.PrintDefaults()
	} else {
		/* set the server ready for use */
		// confirm the html templates
		ui.InitializeTemplates(templatesFolder)

		// coordinates for connecting to the sqlite database (from the command line options)
		dbCoordinates := database.ConnCoordinates{DBPath: dbPath, DBFile: dbFile}

		// prepare the apiHost:apiPort for handler functions that need them
		extraCoordinates := make([]interface{}, 1)
		extraCoordinates[0] = fmt.Sprintf("%s:%d", apiHost, apiPort)

		/* define the server handlers */

		// dynamic request handlers: html
		http.HandleFunc("/", ui.Redirect("/stulist/"))
		http.HandleFunc("/browser", ui.UnsupportedBrowserHandler(templatesFolder)) //OK
		http.HandleFunc("/shutdown/", ui.ShutdownClientHandler()) //OK
		http.HandleFunc("/stulist/", ui.MakeHTMLHandler(ui.ScannedItems, dbCoordinates))
		http.HandleFunc("/delete/", ui.MakeHTMLHandler(ui.DeleteItems, dbCoordinates))
		http.HandleFunc("/submit/", ui.MakeHTMLHandler(ui.FavoriteItems, dbCoordinates))
		http.HandleFunc("/unsubmit/", ui.MakeHTMLHandler(ui.UnfavoriteItems, dbCoordinates))
		http.HandleFunc("/input/", ui.MakeHTMLHandler(ui.InputUnknownItem, dbCoordinates, extraCoordinates...))

		// ajax
		http.HandleFunc("/remove/", ui.MakeHandler(ui.RemoveSingleItem, dbCoordinates, MIME_JSON))
		http.HandleFunc("/status/", ui.MakeHandler(ui.ConfirmServerAccount, dbCoordinates, MIME_JSON, extraCoordinates...))

		// static resources
		http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir(path.Join(templatesFolder, "../css/")))))
		http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(path.Join(templatesFolder, "../js/")))))
		http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir(path.Join(templatesFolder, "../fonts/")))))
		http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(path.Join(templatesFolder, "../images/")))))

		/* start the server */
		log.Println(fmt.Sprintf("Starting the WebApp %s", fmt.Sprintf("%s:%d", host, port)))
		log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil))
	}
}
