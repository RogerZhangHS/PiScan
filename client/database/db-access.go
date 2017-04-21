// Copyright Banrai LLC. All rights reserved. Use of this source code is
// governed by the license that can be found in the LICENSE file.

// Package database provides access to the sqlite database on the Pi client

package database

import (
	"fmt"
	"github.com/Banrai/PiScan/server/database/barcodes"
	"github.com/mxk/go-sqlite/sqlite3"
	"io/ioutil"
	"math"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	// Default database filename
	SQLITE_PATH = "/data"
	SQLITE_FILE = "PiScanDB.sqlite"

	// Default sql definitions file
	TABLE_SQL_DEFINITIONS = "tables.sql"

	// Execution constants
	BAD_PK = -1

	// Prepared Statements
	// Students
	ADD_STUDENT          = "insert into Student (stuid, name, submission_status, submission_time) values ($b, $n, $i, $e)" //EDITED
	UPDATE_STUDENT       = "update Student set stuid = $d, name = $n where stuid = $i" //EDITED
	GET_EXISTING_STUDENT = "select stuid from Student where stuid = $b" //EDITED
	DELETE_STUDENT       = "delete from Student where stuid = $i"
	SIGN_STUDENT         = "update Student set submission_status = 1 where stuid = $i"
	UNSIGN_STUDENT       = "update Student set submission_status = 0 where stuid = $i"
)

var (
	INTERVALS   = []string{"year", "month", "day", "hour", "minute"}
	SECONDS_PER = map[string]int64{"minute": 60, "hour": 3600, "day": 86400, "month": 2592000, "year": 31536000}
)

func calculateTimeSince(posted string) string {
	result := "just now" // default reply

	// try to convert the posted string into unix time
	i, err := strconv.ParseInt(posted, 10, 64)
	if err == nil {
		tm := time.Unix(i, 0)

		// calculate the time since posted
		// and return a human readable
		// '[interval] ago' string
		duration := time.Since(tm)
		if duration.Seconds() < 60.0 {
			if duration.Seconds() == 1.0 {
				result = fmt.Sprintf("%2.0f second ago", duration.Seconds())
			} else {
				result = fmt.Sprintf("%2.0f seconds ago", duration.Seconds())
			}
		} else {
			for _, interval := range INTERVALS {
				v := math.Trunc(duration.Seconds() / float64(SECONDS_PER[interval]))
				if v > 0.0 {
					if v == 1.0 {
						result = fmt.Sprintf("%2.0f %s ago", v, interval)
					} else {
						// plularize the interval label
						result = fmt.Sprintf("%2.0f %ss ago", v, interval)
					}
					break
				}
			}
		}
	}

	return result
}

func getPK(db *sqlite3.Conn, table string) int64 {
	// find and return the most recently-inserted
	// primary key, based on the table name
	sql := fmt.Sprintf("select seq from sqlite_sequence where name='%s'", table)

	var rowid int64
	for s, err := db.Query(sql); err == nil; err = s.Next() {
		s.Scan(&rowid)
	}
	return rowid
}


type Student struct {
	name string
	stuid string
	submission_status bool
	submission_time int64
}

func getExistingItem(db *sqlite3.Conn, barcode string) int64 {
	// lookup the stuid
	// combination and return the primary key,
	// if the product has already been saved

	args := sqlite3.NamedArgs{"$b": barcode}

	var rowid int64
	rowid = BAD_PK // default value, in case no match
	for s, err := db.Query(GET_EXISTING_STUDENT, args); err == nil; err = s.Next() {
		s.Scan(&rowid)
	}
	return rowid
}

func (i *Student) Add(db *sqlite3.Conn) (int64, error) {
	// insert the Item object

	// but first check if it's a duplicate or not
	itemPk := getExistingItem(db, i.stuid)
	if itemPk != BAD_PK {
		return itemPk, nil
	}

	args := sqlite3.NamedArgs{"$b": i.stuid,
		"$n": i.name,
		"$i": i.submission_status,
		"$e": i.submission_time}
	result := db.Exec(ADD_STUDENT, args)
	//TODO: INTERPRET THE FOLLOWING CODE
	if result == nil {
		pk := getPK(db, "product")
		return pk, result
	}

	return BAD_PK, result
}

func (i *Student) Update(db *sqlite3.Conn, original_stuid string) error {
	// 更新学生的个人信息
	args := sqlite3.NamedArgs{"$d": i.stuid,
		"$n": i.name,
		"$i": original_stuid}
	return db.Exec(UPDATE_STUDENT, args)
}

func (i *Student) Delete(db *sqlite3.Conn) error {
	// 删除学生的个人信息
	args := sqlite3.NamedArgs{"$i": i.stuid}
	return db.Exec(DELETE_STUDENT, args)
}

func (i *Student) Sign(db *sqlite3.Conn) error {
	// 更改上交作业的状态为 True
	args := sqlite3.NamedArgs{"$i": i.stuid}
	return db.Exec(SIGN_STUDENT, args)
}

func (i *Student) Unsign(db *sqlite3.Conn) error {
	// 更改上交作业的状态为 False
	args := sqlite3.NamedArgs{"$i": i.stuid}
	return db.Exec(UNSIGN_STUDENT, args)
}

func InitializeDB(coords ConnCoordinates) (*sqlite3.Conn, error) {
	// attempt to open the sqlite db file
	db, dbErr := sqlite3.Open(path.Join(coords.DBPath, coords.DBFile))
	if dbErr != nil {
		return db, dbErr
	}

	// load the table definitions file, if coords.DBTablesPath is defined
	if len(coords.DBTablesPath) > 0 {
		content, err := ioutil.ReadFile(path.Join(coords.DBTablesPath, TABLE_SQL_DEFINITIONS))
		if err != nil {
			return db, err
		}

		// attempt to create (if not exists) each table
		tables := strings.Split(string(content), ";")
		for _, table := range tables {
			err = db.Exec(table)
			if err != nil {
				return db, err
			}
		}
	}

	return db, nil
}
