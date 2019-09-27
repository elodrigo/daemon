package message

func MapInsertPostgres(m map[string]string) string {
	var queryString string

	table := m["table_name"]
	delete(m, "table_name")

	var columnNames string
	var columnValues string
	columnNames = " "
	columnValues = " "

	for key, val := range m {
		columnNames += key + ","
		columnValues += "'" + val + "',"
	}

	columnNames = columnNames[:len(columnNames)-1]
	columnValues = columnValues[:len(columnValues)-1]
	queryString = "INSERT INTO " + table + "(" + columnNames + ")" + " VALUES (" + columnValues + ")"
	return queryString
}
