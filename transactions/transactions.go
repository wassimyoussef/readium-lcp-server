package transactions

import (
	"database/sql"
	"errors"
	"time"

	"github.com/readium/readium-lcp-server/status"
)

var NotFound = errors.New("Event not found")

type Transactions interface {
	Get(id int) (Event, error)
	Add(e Event, typeEvent int) error
	GetByLicenseStatusId(licenseStatusFk int) func() (Event, error)
	CheckDeviceStatus(licenseStatusFk int, deviceId string) (string, error)
	ListRegisteredDevices(licenseStatusFk int) func() (Device, error)
}

type RegisteredDevicesList struct {
	Id      string   `json:"id"`
	Devices []Device `json:"devices"`
}

type Device struct {
	DeviceId   string    `json:"id"`
	DeviceName string    `json:"name"`
	Timestamp  time.Time `json:"timestamp"`
}

type Event struct {
	Id              int       `json:"-"`
	DeviceName      string    `json:"name"`
	Timestamp       time.Time `json:"timestamp"`
	Type            string    `json:"type"`
	DeviceId        string    `json:"id"`
	LicenseStatusFk int       `json:"-"`
}

type dbTransactions struct {
	db                    *sql.DB
	get                   *sql.Stmt
	add                   *sql.Stmt
	getbylicensestatusid  *sql.Stmt
	checkdevicestatus     *sql.Stmt
	listregistereddevices *sql.Stmt
}

func (i dbTransactions) Get(id int) (Event, error) {
	records, err := i.get.Query(id)
	var typeInt int

	defer records.Close()
	if records.Next() {
		var e Event
		err = records.Scan(&e.Id, &e.DeviceName, &e.Timestamp, &typeInt, &e.DeviceId, &e.LicenseStatusFk)
		if err == nil {
			e.Type = status.Types[typeInt]
		}
		return e, err
	}

	return Event{}, NotFound
}

func (i dbTransactions) Add(e Event, typeEvent int) error {
	add, err := i.db.Prepare("INSERT INTO event VALUES (?, ?, ?, ?, ?, ?)")

	if err != nil {
		return err
	}

	defer add.Close()
	_, err = add.Exec(nil, e.DeviceName, e.Timestamp, typeEvent, e.DeviceId, e.LicenseStatusFk)
	return err
}

func (i dbTransactions) GetByLicenseStatusId(licenseStatusFk int) func() (Event, error) {
	rows, err := i.getbylicensestatusid.Query(licenseStatusFk)
	if err != nil {
		return func() (Event, error) { return Event{}, err }
	}
	return func() (Event, error) {
		var e Event
		var err error
		if rows.Next() {
			err = rows.Scan(&e.Id, &e.DeviceName, &e.Timestamp, &e.Type, &e.DeviceId, &e.LicenseStatusFk)
		} else {
			rows.Close()
			err = NotFound
		}
		return e, err
	}
}

func (i dbTransactions) ListRegisteredDevices(licenseStatusFk int) func() (Device, error) {
	rows, err := i.listregistereddevices.Query(licenseStatusFk)
	if err != nil {
		return func() (Device, error) { return Device{}, err }
	}
	return func() (Device, error) {
		var d Device
		var err error
		if rows.Next() {
			err = rows.Scan(&d.DeviceId, &d.DeviceName, &d.Timestamp)
		} else {
			rows.Close()
			err = NotFound
		}
		return d, err
	}
}

func (i dbTransactions) CheckDeviceStatus(licenseStatusFk int, deviceId string) (string, error) {
	var typeString string
	var typeInt int64

	row := i.checkdevicestatus.QueryRow(licenseStatusFk, deviceId)
	err := row.Scan(&typeInt)

	if err != nil {
		status.GetStatus(typeInt, &typeString)
	}

	return typeString, err
}

func Open(db *sql.DB) (t Transactions, err error) {
	_, err = db.Exec(tableDef)
	if err != nil {
		return
	}
	get, err := db.Prepare("SELECT * FROM event WHERE id = ? LIMIT 1")
	if err != nil {
		return
	}

	getbylicensestatusid, err := db.Prepare("SELECT * FROM event WHERE license_status_fk = ?")

	checkdevicestatus, err := db.Prepare(`SELECT type FROM event WHERE license_status_fk = ?
	AND device_id = ? ORDER BY timestamp DESC LIMIT 1`)

	listregistereddevices, err := db.Prepare(`SELECT device_id, 
	device_name, timestamp  FROM event  WHERE license_status_fk = ? AND type = 1`)

	if err != nil {
		return
	}

	t = dbTransactions{db, get, nil, getbylicensestatusid, checkdevicestatus, listregistereddevices}
	return
}

const tableDef = `CREATE TABLE IF NOT EXISTS event (
	id integer PRIMARY KEY, 
	device_name varchar(255) DEFAULT NULL,
	timestamp datetime NOT NULL,
	type int NOT NULL,
	device_id varchar(255) DEFAULT NULL,
	license_status_fk int(11) NOT NULL )`
