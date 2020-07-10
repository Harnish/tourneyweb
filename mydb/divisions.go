package mydb

import (
	"log"
	"strconv"
)

type Division struct {
	ID   int
	Name string
}

func (me *MyDB) AddDivision(name string) {

	query := "Insert into DIVISIONS (DivisionName) values (?);"

	statement, err := me.DB.Prepare(query)
	if err != nil {
		log.Println("Error - AddDivision - Prepare ", err)
		return
	}
	_, err = statement.Exec(name)
	if err != nil {
		log.Println("Error - AddDivison writing ", err)
		_, err = statement.Exec(name)
	}
}

func (me *MyDB) DelDivision(id int) {
	query := "delete from DIVISIONS where id=" + strconv.Itoa(id) + ";"
	log.Println(query)
	me.DB.Exec(query)

}

func (me *MyDB) ReturnDivisions() (alldivisions []Division) {

	query := "Select id, DivisionName from DIVISIONS;"

	rows, err := me.DB.Query(query)
	var temp Division
	if err != nil {
		log.Println("Error - ReturnDivisons - Query ", err, query)
		return
	}
	for rows.Next() {
		rows.Scan(&temp.ID, &temp.Name)

		alldivisions = append(alldivisions, temp)

	}
	rows.Close()
	return
}

func (me *MyDB) ReturnDivisionByID(id int) (division Division) {

	query := "Select id, DivisionName from DIVISIONS where id=" + strconv.Itoa(id) + ";"

	rows, err := me.DB.Query(query)
	if err != nil {
		log.Println("Error - ReturnDivisonByID - Query ", err, query)
		return
	}
	for rows.Next() {
		rows.Scan(&division.ID, &division.Name)
		rows.Close()

	}
	return
}
