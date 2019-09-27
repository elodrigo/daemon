package messagequeue

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

func GetSelectQueryString(msgRecv []byte) (string, string) {

	ID, table := GetIDAndTableName(msgRecv)
	log.Printf("Received message: id=%q, table=%q\n", ID, table)

	confArray, err := GetConfArray(string(table))
	if err != nil {
		log.Println(err)
	}

	confString := strings.Join(confArray, ",")

	queryString := fmt.Sprintf("SELECT %s FROM %s WHERE id=%s", confString, table, ID)

	return queryString, string(table)
}

func GetQueryResult(rows *sql.Rows) map[string]string {

	columnNames, err := rows.Columns()
	if err != nil {
		log.Println(err)
	}
	rc := NewMapStringScan(columnNames)

	var cv map[string]string

	for rows.Next() {
		err := rc.Update(rows)
		if err != nil {
			log.Println(err)
		}
		cv = rc.Get()
		// log.Printf("%#v", cv)
	}
	return cv
	// var ctx = context.Background()

	// row := DBPostgres.QueryRowContext(ctx, queryString)

	// rows, sql_err := DBPostgres.QueryContext(ctx, queryString)
	// if sql_err != nil {
	// 	log.Println(sql_err)
	// }
	// cols, err := rows.Columns()
	// if err != nil {
	// 	log.Println(err)
	// }
	// log.Println(cols)
	// vals := make([]interface{}, len(cols))
	// for i, _ := range cols {
	// 	vals[i] = new(sql.RawBytes)
	// }
	// for rows.Next() {
	// 	err = rows.Scan(vals...)

	// }
	// log.Println(ctx)
}

type mapStringScan struct {

	// cp are the column pointers

	cp []interface{}

	// row contains the final result

	row map[string]string

	colCount int

	colNames []string
}

func NewMapStringScan(columnNames []string) *mapStringScan {

	lenCN := len(columnNames)

	s := &mapStringScan{

		cp: make([]interface{}, lenCN),

		row: make(map[string]string, lenCN),

		colCount: lenCN,

		colNames: columnNames,
	}

	for i := 0; i < lenCN; i++ {

		s.cp[i] = new(sql.RawBytes)

	}

	return s

}

func (s *mapStringScan) Update(rows *sql.Rows) error {

	if err := rows.Scan(s.cp...); err != nil {

		return err

	}

	for i := 0; i < s.colCount; i++ {

		if rb, ok := s.cp[i].(*sql.RawBytes); ok {

			s.row[s.colNames[i]] = string(*rb)

			*rb = nil // reset pointer to discard current value to avoid a bug

		} else {

			return fmt.Errorf("Cannot convert index %d column %s to type *sql.RawBytes", i, s.colNames[i])

		}

	}

	return nil

}

func (s *mapStringScan) Get() map[string]string {

	return s.row

}
