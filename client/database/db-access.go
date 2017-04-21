// Copyright Banrai LLC. All rights reserved. Use of this source code is
// governed by the license that can be found in the LICENSE file.

// Package database provides access to the sqlite database on the Pi client

package database

import (
	"fmt"
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
	ADD_STUDENT           = "insert into Student (stuid, name) values ($b, $n)" //EDITED
	UPDATE_STUDENT        = "update Student set stuid = $d, name = $n where stuid = $i" //EDITED
	GET_EXISTING_STUDENT  = "select stuid from Student where stuid = $b" //EDITED
	DELETE_STUDENT        = "delete from Student where stuid = $i"
	SUBMITTED_STUDENT     = "update Student set submission_status = 1 where stuid = $i"
	UNSUBMITTED_STUDENT   = "update Student set submission_status = 0 where stuid = $i"
	GET_SUBMITTED_STUDENT = "select name, stuid, submission_time, submission_status from Student where submission_status = 1 order by posted submission_time"
	GET_STUDENT           = "select name, stuid, submission_time, submission_status from Student order by posted submission_time"
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

type ConnCoordinates struct {
	DBPath string
	DBFile string
	DBTablesPath string
}

func getExistingStudent(db *sqlite3.Conn, barcode string) int64 {
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

func (i *Student) Add(db *sqlite3.Conn, stuid, name string) (int64, error) {
	// 新增一个学生

	// 首先检查是否与现存数据库内容重复
	itemPk := getExistingStudent(db, i.stuid)
	if itemPk != BAD_PK {
		return itemPk, nil
	}

	args := sqlite3.NamedArgs{"$b": i.stuid,
		"$n": i.name}
	result := db.Exec(ADD_STUDENT, args)
	//TODO: INTERPRET THE FOLLOWING CODE
	if result == nil {
		pk := getPK(db, "product")
		return pk, result
	}

	return BAD_PK, result
}

func (i *Student) Update(db *sqlite3.Conn, original_stuid, stuid, name string) error {
	// 更新学生的个人信息
	args := sqlite3.NamedArgs{"$d": i.stuid,
		"$n": i.name,
		"$i": original_stuid}
	return db.Exec(UPDATE_STUDENT, args)
}

func (i *Student) Delete(db *sqlite3.Conn, stuid string) error {
	// 删除学生的个人信息
	args := sqlite3.NamedArgs{"$i": i.stuid}
	return db.Exec(DELETE_STUDENT, args)
}

func (i *Student) Submit(db *sqlite3.Conn, stuid string) error {
	// 更改上交作业的状态为 True
	args := sqlite3.NamedArgs{"$i": i.stuid}
	return db.Exec(SUBMITTED_STUDENT, args)
}

func (i *Student) Unsubmit(db *sqlite3.Conn, stuid string) error {
	// 更改上交作业的状态为 False
	args := sqlite3.NamedArgs{"$i": i.stuid}
	return db.Exec(UNSUBMITTED_STUDENT, args)
}

func GetStudents(db *sqlite3.Conn, sql string) ([]*Student, error) {
	results := make([]*Student, 0)
	row := make(sqlite3.RowMap)
	for s, err := db.Query(sql); err == nil; err = s.Next() {
		var rowid int64
		s.Scan(&rowid, row)
		stuid, stuidFound := row["stuid"]
		name := row["name"]
		submission_time := row["submission_time"]
		submission_status := row["submission_status"]
		if stuidFound {
			result := new(Student)
			result.stuid = stuid.(string)
			result.name = name.(string)
			result.submission_time = submission_time.(int64)
			result.submission_status = submission_status.(bool)
			results = append(results,result)
		}
	}

	return results, nil
}

func GetSubmittedStudents(db *sqlite3.Conn) ([]*Student, error) {
	return GetStudents(db, GET_SUBMITTED_STUDENT)
}

func GetAllStudents (db *sqlite3.Conn) ([]*Student, error) {
	return GetStudents(db, GET_STUDENT)
}

func GetSingleItem(db *sqlite3.Conn, id string) (*Student, error) {
	student := new(Student)
	student.stuid = "-1" // if not found
	students, err := GetAllStudents(db)
	for _, i := range students {
		if i.stuid == id {
			return i, err
		}
	}
	return student, err
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
