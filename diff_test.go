// sysinfo_influxdb by Novaquark
//
// To the extent possible under law, the person who associated CC0 with
// sysinfo_influxdb has waived all copyright and related or neighboring rights
// to sysinfo_influxdb.
//
// You should have received a copy of the CC0 legalcode along with this
// work.  If not, see <http://creativecommons.org/publicdomain/zero/1.0/>.

package main

import (
	"fmt"
	influxClient "github.com/influxdb/influxdb/client"
	"math/rand"
	"testing"
)

func TestSimple(t *testing.T) {
	serie1 := &influxClient.Series{
		Name:    "test_init",
		Columns: []string{"col0"},
		Points:  [][]interface{}{[]interface{}{42}},
	}

	if DiffFromLast(serie1) != nil {
		t.Error("First time, initialization can't return a valid serie")
	}

	if serie1.Name != "test_init" {
		t.Error("DiffFromLast shouldn't modify serie name")
	}

	if len(serie1.Columns) != 1 {
		t.Error("DiffFromLast shouldn't modify columns number")
	}

	if serie1.Columns[0] != "col0" {
		t.Error("DiffFromLast shouldn't modify columns name")
	}

	serie2 := &influxClient.Series{
		Name:    "test_init_other",
		Columns: []string{"col0", "col1"},
		Points:  [][]interface{}{[]interface{}{23, 22}, []interface{}{33, 32}},
	}

	if DiffFromLast(serie2) != nil {
		t.Error("Another serie (different serie name) have to be initialized too")
	}

	serie1.Points = [][]interface{}{[]interface{}{23}}
	serie2.Points = [][]interface{}{[]interface{}{43, 42}, []interface{}{32, 33}}

	if DiffFromLast(serie1) == nil || DiffFromLast(serie2) == nil {
		t.Error("Initialized diff serie shouldn't return nil")
	}

	if serie1.Points[0][0] != 23-42 {
		t.Error("Bad diff:", serie1.Points[0][0], "!= 23 - 42")
	}

	if serie2.Points[0][0] != 43-23 {
		t.Error("Bad diff:", serie2.Points[0][0], "!= 43 - 23")
	}
	if serie2.Points[0][1] != 42-22 {
		t.Error("Bad diff:", serie2.Points[0][1], "!= 42 - 22")
	}
	if serie2.Points[1][0] != 32-33 {
		t.Error("Bad diff:", serie2.Points[1][0], "!= 32 - 33")
	}
	if serie2.Points[1][1] != 33-32 {
		t.Error("Bad diff:", serie2.Points[1][1], "!= 33 - 32")
	}
}

func fillPoints(serie *influxClient.Series, pts *[][]interface{}, sizeX int, sizeY int) {
	for i := 0; i < sizeX; i++ {
		if len(serie.Columns) <= i {
			serie.Columns = append(serie.Columns, fmt.Sprintf("col%d", i))
			serie.Points = append(serie.Points, []interface{}{})
		}
		*pts = append(*pts, []interface{}{})

		for j := 0; j < sizeY; j++ {
			tmp := rand.Int()
			(*pts)[i] = append((*pts)[i], tmp)
			if len(serie.Points[i]) <= j {
				serie.Points[i] = append(serie.Points[i], tmp)
			} else {
				serie.Points[i][j] = tmp
			}
		}
	}
}

func TestRandom(t *testing.T) {
	serie := &influxClient.Series{
		Name:    fmt.Sprintf("test_rnd"),
		Columns: []string{},
		Points:  [][]interface{}{},
	}

	sizeX := rand.Intn(20) + 10
	sizeY := rand.Intn(20) + 10

	var oldPts, newPts *[][]interface{}
	newPts = new([][]interface{})

	fillPoints(serie, newPts, sizeX, sizeY)

	DiffFromLast(serie)

	for h := 0; h < rand.Intn(50)+10; h++ {
		oldPts = newPts
		newPts = new([][]interface{})

		fillPoints(serie, newPts, sizeX, sizeY)
		DiffFromLast(serie)

		// Compare
		for i := 0; i < sizeX; i++ {
			for j := 0; j < sizeY; j++ {
				if serie.Points[i][j] != (*newPts)[i][j].(int)-(*oldPts)[i][j].(int) {
					t.Error(fmt.Sprintf("Iteration %d; point %d,%d: expected %d, got %d", h, i, j,
						(*newPts)[i][j].(int)-(*oldPts)[i][j].(int), serie.Points[i][j]))
				}
			}
		}
	}
}
