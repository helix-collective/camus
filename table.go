package main

import (
	"fmt"
	"strconv"
)

type ColumnDef struct {
	Heading string
	Size    int
}

type TableDef struct {
	Columns []ColumnDef
}

func (t *TableDef) PrintHeader() {
	for _, col := range t.Columns {
		fmt.Printf("%-"+strconv.Itoa(col.Size)+"s ", col.Heading)
		//fmt.Printf("%-"+string(col.Size)+"s ", col.Heading)
	}
	println()
}

func (t *TableDef) PrintRow(cells ...interface{}) {
	lines := make([][]string, 0, 10)
	for i, col := range t.Columns {
		str := format(col.Size, cells[i])

		num := 0
		for len(str) > 0 {
			if num >= len(lines) {
				lines = append(lines, make([]string, len(cells)))
			}

			str = fmt.Sprintf(formatStr(col.Size), str)

			lines[num][i] = str[0:col.Size]
			str = str[col.Size:]

			num++
		}

		//fmt.Printf("%-"+strconv.Itoa(col.Size)+"s ", cells[i])
	}
	for _, line := range lines {
		for i, cell := range line {
			if len(cell) == 0 {
				cell = fmt.Sprintf(formatStr(t.Columns[i].Size), "")
			}
			print(cell)
			print(" ")
		}
		println()
	}
}

func formatStr(l int) string {
	return "%-" + strconv.Itoa(l) + "s"
}

func format(colSize int, v interface{}) string {
	l := strconv.Itoa(colSize)

	switch v := v.(type) {
	default:
		return fmt.Sprintf("%"+l+"v", v)
	case string:
		return fmt.Sprintf("%-"+l+"s", v)
	case bool:
		return fmt.Sprintf("%"+l+"t", v)
	case int:
		return fmt.Sprintf("%"+l+"d", v)
	case *bool:
		return fmt.Sprintf("%"+l+"t", *v)
	case *int:
		return fmt.Sprintf("%"+l+"d ", *v)
	}
}
