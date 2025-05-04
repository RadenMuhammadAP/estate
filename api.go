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
		name TEXT,
		width INT,
		length INT		
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
	err := db.QueryRow("SELECT width, length FROM estates WHERE id = $1", estateID).Scan(&width,&length)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "estate not found"})
	}

	if req.X < 1 || req.X > length || req.Y < 1 || req.Y > width {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tree position"})
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

func calculateMedian(data []int) float64 {
    sort.Ints(data)
    n := len(data)
    if n%2 == 0 {
        return float64(data[n/2-1]+data[n/2]) / 2.0
    }
    return float64(data[n/2])
}

func checkMaxDistance(c echo.Context, distance, max, x, y int) (bool, error){
	if max > 0 && distance > max {
		return true,c.JSON(http.StatusOK, map[string]interface{}{
			"rest":    map[string]int{"x": x, "y": y},
			"distance": max,
		})
	}else{
		return false, nil			
	}
}

func getDronePlan(c echo.Context) error {
	treeMap := make(map[[2]int]int)
	estateID := c.Param("id")
	maxDistanceInt := 0
	maxDistance := c.QueryParam("max_distance")
	var err error
	var shouldBreak bool	
	if maxDistance != "" {
		maxDistanceInt, err = strconv.Atoi(maxDistance)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid max-distance"})
		}
	}	
	var width, length int
	err = db.QueryRow("SELECT width, length FROM estates WHERE id = $1", estateID).Scan(&width, &length)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "estates not found"})
	}
	rows, err := db.Query("SELECT x, y, height FROM trees WHERE estate_id = $1 ORDER BY id", estateID)
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
	horizontally := 0;
	vertically :=0;
	for y := 1; y <= width; y++ {				
		if y%2 == 1 {
			for x := 1; x <= length; x++ {
				if(x == 1 && y == 1){
					totalDistance += 1;
					vertically += 1;	
					fmt.Println(vertically," meter vertically, total distance :",totalDistance)										
					nextX = x + 1
					if height, ok := treeMap[[2]int{nextX, y}]; ok {
						fmt.Println("there is a next tree on x=",nextX,"y=",y," with height=",height)
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						totalDistance += height;
						vertically += height;
						fmt.Println(vertically," meter vertically, total distance :",totalDistance)																
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
					}																			
				}else{
					totalDistance += 10;
					horizontally += 10;	
					fmt.Println(horizontally," meter horizontally, totalDistance=",totalDistance)
					shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
					if err != nil {
						return err
					}
					if shouldBreak {
						break
					}
					fmt.Println("now,the position is x=",x,"y=",y)															
					nextX = x + 1
					if height, ok := treeMap[[2]int{x, y}]; ok {
						if height2, ok := treeMap[[2]int{nextX, y}]; ok {
							fmt.Println("there is a next tree on x=",nextX," y=",y," with height=",height)
							totalDistance += abs(height - height2);
							vertically += abs(height - height2);
							fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)																																														
							shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
							if err != nil {
								return err
							}
							if shouldBreak {
								break
							}
						}else if(nextX == lastX && y ==lastY){
						    fmt.Println("now,the next is the last position on x=",nextX," y=",y)																						
							totalDistance += 10;
							horizontally += 10;	
							shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
							if err != nil {
								return err
							}
							if shouldBreak {
								break
							}							
							x = lastX;
							y = lastY;							
							fmt.Println(horizontally," meter horizontally, totalDistance=",totalDistance)
							totalDistance += height;
							vertically += height;	
							fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)																																							
							shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
							if err != nil {
								return err
							}
							if shouldBreak {
								break
							}
							totalDistance += 1;
							vertically += 1;
						    fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)																																								
							shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
							if err != nil {
								return err
							}
							if shouldBreak {
								break
							}
							break;
						}else{
							totalDistance += height;							
						}
					}else if height, ok := treeMap[[2]int{nextX, y}]; ok {
						fmt.Println("now,there is a next tree on x=",nextX," y=",y)																						
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						totalDistance += height;
						vertically += height;	
						fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)							
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
					}else if(nextX == lastX && y ==lastY){
						totalDistance += 10;																													
						horizontally += 10;	
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						x = lastX;
						y = lastY;							
						fmt.Println("now,the next is 2 the last position on x=",x," y=",y)																						
						fmt.Println(horizontally," meter horizontally, totalDistance=",totalDistance)							
						totalDistance += height;							
						vertically += height;	
						fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						vertically += 1;
						totalDistance += 1;
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)							
						break;
					}	
				}
			}
		}else {
			if shouldBreak {
				break
			}			
			for x := length; x >= 1; x-- {					
				totalDistance += 10;
				horizontally += 10;	
				fmt.Println(horizontally," meter horizontally, totalDistance=",totalDistance)
				shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
				if err != nil {
					return err
				}
				if shouldBreak {
					break
				}
				fmt.Println("now,the position is x=",x,"y=",y)															
				nextX = x - 1
				if height, ok := treeMap[[2]int{x, y}]; ok {
					if height2, ok := treeMap[[2]int{nextX, y}]; ok {
						fmt.Println("there is a next tree on x=",nextX," y=",y," with height=",height)
						totalDistance += abs(height - height2);
						vertically += abs(height - height2);
						fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)																																														
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
					}else if(nextX == lastX && y ==lastY){
						fmt.Println("now,the next is the last position on x=",nextX," y=",y)																						
						totalDistance += 10;
						horizontally += 10;	
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}							
						x = lastX;
						y = lastY;							
						fmt.Println(horizontally," meter horizontally, totalDistance=",totalDistance)
						totalDistance += height;
						vertically += height;	
						fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)																																							
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						totalDistance += 1;
						vertically += 1;
						fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)																																								
						shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
						if err != nil {
							return err
						}
						if shouldBreak {
							break
						}
						break;
					}else{
						totalDistance += height;							
					}
				}else if height, ok := treeMap[[2]int{nextX, y}]; ok {
					fmt.Println("now,there is a next tree on x=",nextX," y=",y)																						
					shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
					if err != nil {
						return err
					}
					if shouldBreak {
						break
					}
					totalDistance += height;
					vertically += height;	
					fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)							
					shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
					if err != nil {
						return err
					}
					if shouldBreak {
						break
					}
				}else if(nextX == lastX && y ==lastY){
					totalDistance += 10;																													
					horizontally += 10;	
					shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
					if err != nil {
						return err
					}
					if shouldBreak {
						break
					}
					x = lastX;
					y = lastY;							
					fmt.Println("now,the next is 2 the last position on x=",x," y=",y)																						
					fmt.Println(horizontally," meter horizontally, totalDistance=",totalDistance)							
					totalDistance += height;							
					vertically += height;	
					fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)
					shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
					if err != nil {
						return err
					}
					if shouldBreak {
						break
					}
					vertically += 1;
					totalDistance += 1;
					shouldBreak, err = checkMaxDistance(c, totalDistance, maxDistanceInt, x, y)
					if err != nil {
						return err
					}
					if shouldBreak {
						break
					}
					fmt.Println(vertically," meter vertically, totalDistance=",totalDistance)							
					break;
				}								
			}							
		}
	}
	if shouldBreak {
		return nil
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
