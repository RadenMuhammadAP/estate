package main

import (
	"fmt"
	"database/sql"
	"log"
	"net/http"
	"sort"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
    "strconv"
)

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("postgres", "user=postgres password=postgres dbname=estate_db search_path=public sslmode=disable")
	if err != nil {
		log.Fatal("DB connection error:", err)
	}
	if err := createTables(); err != nil {
		log.Fatal("Failed creating tables:", err)
	}	
}

func createTables() error {
	estateTable := `
	CREATE TABLE IF NOT EXISTS estates (
		id UUID PRIMARY KEY,
		name TEXT
	);`

	treeTable := `
	CREATE TABLE IF NOT EXISTS trees (
		id UUID PRIMARY KEY,
		estate_id UUID REFERENCES estates(id),
		x INT,
		y INT,
		height INT,
		UNIQUE(estate_id, x, y)
	);`

	if _, err := db.Exec(estateTable); err != nil {
		return fmt.Errorf("failed to create estates table: %w", err)
	}

	if _, err := db.Exec(treeTable); err != nil {
		return fmt.Errorf("failed to create trees table: %w", err)
	}

	return nil
}


type Estate struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Length int    `json:"length"`
}

type Tree struct {
	ID     string `json:"id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Height int    `json:"height"`
}

func createEstate(c echo.Context) error {
	var req Estate
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input"})
	}

	if req.Width < 1 || req.Width > 50000 || req.Length < 1 || req.Length > 50000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid estate size"})
	}
	
	id := uuid.New().String()
	_, err := db.Exec("INSERT INTO estates (id, width, length) VALUES ($1, $2, $3)", id, req.Width, req.Length)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}	
	return c.JSON(http.StatusOK, map[string]string{"id": id})
}

func addTree(c echo.Context) error {
	estateID := c.Param("id")
	var req Tree
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input"})
	}

	var width, length int
	err := db.QueryRow("SELECT length, width FROM estates WHERE id = $1", estateID).Scan(&width, &length)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "estate not found"})
	}

	if req.X < 0 || req.X >= length || req.Y < 0 || req.Y >= width || req.Height < 1 || req.Height > 30 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tree position or height"})
	}

	treeID := uuid.New().String()
	_, err = db.Exec("INSERT INTO trees (id, estate_id, x, y, height) VALUES ($1, $2, $3, $4, $5)", treeID, estateID, req.X, req.Y, req.Height)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to insert tree"})
	}

	return c.JSON(http.StatusOK, map[string]string{"id": treeID})
}

func getEstateStats(c echo.Context) error {
	estateID := c.Param("id")
	rows, err := db.Query("SELECT height FROM trees WHERE estate_id = $1", estateID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query error"})
	}
	defer rows.Close()

	var heights []int
	var height int
	for rows.Next() {
		if err := rows.Scan(&height); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "scan error"})
		}
		heights = append(heights, height)
	}

	if len(heights) == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no trees found"})
	}

	sort.Ints(heights)
	maxHeight := heights[len(heights)-1]
	minHeight := heights[0]
	medianHeight := calculateMedian(heights)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"tree_count":   len(heights),
		"max_height":   maxHeight,
		"min_height":   minHeight,
		"median_height": medianHeight,
	})
}

func calculateMedian(data []int) int {
	if len(data) == 0 {
		return 0
	}
	n := len(data)
	if n%2 == 1 {
		return data[n/2]
	}
	return (data[n/2-1] + data[n/2]) / 2
}

func getDronePlan(c echo.Context) error {
	treeMap := make(map[[2]int]int)
	estateID := c.Param("id")
	maxDistance := c.QueryParam("max-distance")
    maxDistanceInt, err := strconv.Atoi(maxDistance)
	var width, length int
	err = db.QueryRow("SELECT width, length FROM estates WHERE id = $1", estateID).Scan(&width, &length)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "estates not found"})
	}
	rows, err := db.Query("SELECT x, y, height FROM trees WHERE estate_id = $1 ORDER BY y, x", estateID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "trees not found"})
	}
	defer rows.Close()
	for rows.Next() {
		var t Tree
		if err := rows.Scan(&t.X, &t.Y, &t.Height); err == nil {
			treeMap[[2]int{t.X, t.Y}] = t.Height
		}
	}
	totalDistance := 0;
	nextX := 0;
	droneNaik := false;
	lastX := 0;
	lastY := 0;	
	for y := 1; y <= width; y++ {
		if y%2 == 1 {
			for x := 1; x <= length; x++ {
				lastX = x;
				lastY = y;				
			}
		}else{
			for x := length; x >= 1; x-- {
				lastX = x;
				lastY = y;							
			}			
		}
	}
	for y := 1; y <= width; y++ {
		if y%2 == 1 {
			for x := 1; x <= length; x++ {
				if(x == 1 && y == 1){
					fmt.Println("posisi di x=",x,"y=",y)										
					nextX = x + 1
					if height, ok := treeMap[[2]int{nextX, y}]; ok {
						fmt.Println("ada pohon di x=",nextX,"y=",y,"height=",height)
						if(droneNaik == false){
							totalDistance += 1;
							fmt.Println("totalDistance=",totalDistance)	
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}
							droneNaik = true;	
						}	
						totalDistance += height;
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}						
					}	
					fmt.Println("totalDistance=",totalDistance)																			
				}else{
					totalDistance += 10;																						
					fmt.Println("totalDistance=",totalDistance)
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}					
					fmt.Println("posisi di x=",x,"y=",y)															
					nextX = x + 1
					if height, ok := treeMap[[2]int{x, y}]; ok {
						fmt.Println("ada pohon di height=",height)																	
						if height2, ok := treeMap[[2]int{nextX, y}]; ok {
							fmt.Println("pohon ini ",height," ada pohon di x=",nextX,"y=",y,"height=",height2)											
							totalDistance += abs(height - height2);
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}							
						}else if(nextX == lastX && y ==lastY){
							totalDistance += 10;																													
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}							
							x = lastX;
							y = lastY;							
							fmt.Println("totalDistance=",totalDistance)																																
							fmt.Println("posisi di x=",x,"y=",y)																						
							totalDistance += height;							
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}							
							totalDistance += 1;
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}														
							fmt.Println("totalDistance=",totalDistance)																																							
							break;
						}else{
							totalDistance += height;							
						}
					}else if height, ok := treeMap[[2]int{nextX, y}]; ok {
						fmt.Println("ada pohon di x=",nextX,"y=",y,"height=",height)											
						if(droneNaik == false){
							totalDistance += 1;
							if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
		return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
							}														
							fmt.Println("totalDistance=",totalDistance)							
							droneNaik = true;	
						}	
						totalDistance += height;
						if(totalDistance > maxDistanceInt && maxDistanceInt > 0){
	return c.JSON(http.StatusOK, map[string]interface{}{"distance": maxDistanceInt,"rest": map[string]int{"x": x, "y": y}})									
						}													
					}	
					fmt.Println("totalDistance=",totalDistance)																												
				}
			}
		}else {
			for x := length; x >= 1; x-- {
				totalDistance += 10;																						
				fmt.Println("totalDistance=",totalDistance)																																
				fmt.Println("posisi di x=",x,"y=",y)															
				nextX = x - 1
				if height, ok := treeMap[[2]int{x, y}]; ok {
					fmt.Println("ada pohon di height=",height)																	
					if height2, ok := treeMap[[2]int{nextX, y}]; ok {
						fmt.Println("pohon ini ",height," ada pohon di x=",nextX,"y=",y,"height=",height2)											
						totalDistance += abs(height - height2);									
					}else if(nextX == lastX && y ==lastY){
						totalDistance += 10;																													
						x = lastX;
						y = lastY;							
						fmt.Println("totalDistance=",totalDistance)																																
						fmt.Println("posisi di x=",x,"y=",y)																						
						totalDistance += height;							
						totalDistance += 1;
						fmt.Println("totalDistance=",totalDistance)																																							
						break;
					}else{
						totalDistance += height;							
					}
				}else if height, ok := treeMap[[2]int{nextX, y}]; ok {
					fmt.Println("ada pohon di x=",nextX,"y=",y,"height=",height)											
					if(droneNaik == false){
						totalDistance += 1;
						fmt.Println("totalDistance=",totalDistance)							
						droneNaik = true;	
					}	
					totalDistance += height;
				}	
				fmt.Println("totalDistance=",totalDistance)																												
			}							
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"distance": totalDistance})	
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}


func main() {
	initDB()
	e := echo.New()
	e.POST("/estate", createEstate)
	e.POST("/estate/:id/tree", addTree)
	e.GET("/estate/:id/stats", getEstateStats)
	e.GET("/estate/:id/drone-plan", getDronePlan)
	e.Logger.Fatal(e.Start(":8080"))
}
