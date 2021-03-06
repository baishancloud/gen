package dbmeta

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jimsmart/schema"
)

func NewMysqlMeta(db *sql.DB, sqlType, sqlDatabase, tableName string) (DbTableMeta, error) {
	m := &dbTableMeta{
		sqlType:     sqlType,
		sqlDatabase: sqlDatabase,
		tableName:   tableName,
	}

	cols, err := schema.Table(db, m.tableName)
	if err != nil {
		return nil, err
	}

	ddlSql := fmt.Sprintf("SHOW CREATE TABLE %s;", tableName)
	res, err := db.Query(ddlSql)
	if err != nil {
		return nil, fmt.Errorf("unable to load ddl from mysql: %v", err)
	}

	var ddl1 string
	var ddl2 string
	if res.Next() {
		err = res.Scan(&ddl1, &ddl2)
		if err != nil {
			return nil, fmt.Errorf("unable to load ddl from mysql Scan: %v", err)
		}
	}
	m.ddl = ddl2
	//m.ddl = BuildDefaultTableDDL(tableName, cols)

	/*


		CREATE TABLE `designer_work` (
		  `id` int(11) NOT NULL AUTO_INCREMENT,
		  `taskId` int(11) DEFAULT NULL,
		  `userId` int(11) DEFAULT NULL,
		  `image` text,
		  `stickerName` text,
		  `status` int(11) DEFAULT '1' COMMENT '0->reject,1->request sent,2-approved,3-In process',
		  `rejectStatus` int(11) DEFAULT '0' COMMENT '0->not rejected,1-rejected',
		  `rejectedDate` datetime DEFAULT NULL,
		  `reason` text,
		  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,
		  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		  PRIMARY KEY (`id`)
		) ENGINE=InnoDB AUTO_INCREMENT=1622 DEFAULT CHARSET=latin1

	*/

	colsDDL := make(map[string]string)
	lines := strings.Split(m.ddl, "\n")
	primaryKey := ""
	for _, line := range lines {
		line = strings.Trim(line, " \t")
		if strings.HasPrefix(line, "CREATE TABLE") || strings.HasPrefix(line, "(") || strings.HasPrefix(line, ")") {
			continue
		}

		if line[0] == '`' {
			idx := indexAt(line, "`", 1)
			if idx > 0 {
				name := line[1:idx]
				colDDL := line[idx+1 : len(line)-1]
				colsDDL[name] = colDDL
			}
		} else if strings.HasPrefix(line, "PRIMARY KEY") {
			idx := strings.Index(line, "`")
			idx1 := indexAt(line, "`", idx+1)
			primaryKey = line[idx+1 : idx1]
		}
	}

	m.columns = make([]ColumnMeta, len(cols))

	for i, v := range cols {

		nullable, ok := v.Nullable()
		if !ok {
			nullable = false
		}

		colDDL := colsDDL[v.Name()]
		isAutoIncrement := strings.Index(colDDL, "AUTO_INCREMENT") > -1

		isPrimaryKey := v.Name() == primaryKey
		// isPrimaryKey := i == 0
		// colDDL := v.DatabaseTypeName()

		colMeta := &columnMeta{
			index:           i,
			ct:              v,
			nullable:        nullable,
			isPrimaryKey:    isPrimaryKey,
			isAutoIncrement: isAutoIncrement,
			colDDL:          colDDL,
		}
		dbType := strings.ToLower(colMeta.DatabaseTypeName())
		// fmt.Printf("dbType: %s\n", dbType)

		if strings.Contains(dbType, "char") || strings.Contains(dbType, "text") {
			columnLen, err := getFieldLen(db, tableName, v.Name())
			if err == nil {
				colMeta.columnLen = columnLen
				//fmt.Printf("getFieldLen %s %s : columnLen %v\n",tableName,v.Name(), columnLen)
			} else {
				//fmt.Printf("getFieldLen %s %s : error: %v\n",tableName,v.Name(), err)
			}
		}

		m.columns[i] = colMeta
	}
	if err != nil {
		return nil, err
	}

	return m, nil
}
func indexAt(s, sep string, n int) int {
	idx := strings.Index(s[n:], sep)
	if idx > -1 {
		idx += n
	}
	return idx
}

func getFieldLen(db *sql.DB, tableName, columnName string) (int64, error) {
	sql := fmt.Sprintf(`
select CHARACTER_MAXIMUM_LENGTH 
from information_schema.columns
where table_schema = DATABASE() AND   -- name of your database
      table_name = '%s' AND        -- name of your table
      COLUMN_NAME = '%s'     -- name of the column
`, tableName, columnName)

	res, err := db.Query(sql)
	if err != nil {
		return -1, fmt.Errorf("unable to load col len from mysql: %v", err)
	}

	var colLen int64

	if res.Next() {
		err = res.Scan(&colLen)
		if err != nil {
			return -1, fmt.Errorf("unable to load ddl from mysql Scan: %v", err)
		}
	}

	return colLen, nil

}
