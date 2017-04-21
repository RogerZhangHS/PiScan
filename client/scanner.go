// Copyright Banrai LLC. All rights reserved. Use of this source code is
// governed by the license that can be found in the LICENSE file.

// This is a fully-functional (but simple) PiScanner application.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/RogerZhangHS/PiScan/client/database"
	"github.com/RogerZhangHS/PiScan/scanner"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

func main() {
	var (
		device, sqlitePath, sqliteFile, sqliteTablesDefinitionPath string
	)

	flag.StringVar(&device, "device", scanner.SCANNER_DEVICE, fmt.Sprintf("The '/dev/input/event' device associated with your scanner (defaults to '%s')", scanner.SCANNER_DEVICE))
	flag.StringVar(&sqlitePath, "sqlitePath", database.SQLITE_PATH, fmt.Sprintf("Path to the sqlite file (defaults to '%s')", database.SQLITE_PATH))
	flag.StringVar(&sqliteFile, "sqliteFile", database.SQLITE_FILE, fmt.Sprintf("The sqlite database file (defaults to '%s')", database.SQLITE_FILE))
	flag.StringVar(&sqliteTablesDefinitionPath, "sqliteTables", "", fmt.Sprintf("Path to the sqlite database definitions file, %s, (use only if creating the client db for the first time)", database.TABLE_SQL_DEFINITIONS))
	flag.Parse()

	// 连接到本地sqlite数据库
	if len(sqliteTablesDefinitionPath) > 0 {
		// this is a request to create the client db for the first time
		initDb, initErr := database.InitializeDB(database.ConnCoordinates{sqlitePath, sqliteFile, sqliteTablesDefinitionPath})
		if initErr != nil {
			log.Fatal(initErr)
		}
		defer initDb.Close()
		log.Println(fmt.Sprintf("Client database '%s' created in '%s'", sqliteFile, sqlitePath))

	} else {
		// a regular scanner processing event

		// coordinates for connecting to the sqlite database (from the command line options)
		dbCoordinates := database.ConnCoordinates{DBPath: sqlitePath, DBFile: sqliteFile}

		// attempt to connect to the sqlite db
		db, dbErr := database.InitializeDB(dbCoordinates)
		if dbErr != nil {
			log.Fatal(dbErr)
		}
		defer db.Close()

		processScanFn := func(barcode string) {
			// 该函数过程为获取barcode 查询本地数据库中是否存在这些barcode 并且做出相应的反应
			if database.getExistingItem(db, barcode) != -1 {
				database.Sign(db, barcode)
			}
		}

		errorFn := func(e error) {
			log.Fatal(e)
		}

		log.Println(fmt.Sprintf("Starting the scanner %s", device))
		scanner.ScanForever(device, processScanFn, errorFn)
	}
}
