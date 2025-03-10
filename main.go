package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
)

type Point struct {
	Lng float64
	Lat float64
}

type Polygon struct {
	Coordinates [][][]float64 `json:"coordinates"`
	Type        string        `json:"type"`
}

type Feature struct {
	Properties struct {
		Name string `json:"name"`
		Id   string `json:"id"`
	} `json:"properties"`
	Geometry Polygon `json:"geometry"`
	Type     string  `json:"type"`
}

type FeatureCollection struct {
	Features []Feature `json:"features"`
	Type     string    `json:"type"`
}

func main() {
	handler := http.HandlerFunc(geocodeHandler)

	// Start HTTP server (localhost only) in a goroutine
	go func() {
		fmt.Println("HTTP Server listening on :8080 (localhost only)")
		localServer := &http.Server{
			Addr:    "127.0.0.1:8080", // Only accessible from localhost
			Handler: handler,
		}
		if err := localServer.ListenAndServe(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start HTTPS server
	fmt.Println("HTTPS Server listening on :8443")
	err := http.ListenAndServeTLS(":8443",
		"/etc/letsencrypt/live/alpha.bludelivery.et/fullchain.pem",
		"/etc/letsencrypt/live/alpha.bludelivery.et/privkey.pem",
		handler)
	if err != nil {
		log.Fatal("ListenAndServeTLS: ", err)
	}
}

func geocodeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	if r.Method == "OPTIONS" {
		return
	}
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	if latStr == "" || lngStr == "" {
		http.Error(w, "Missing lat or lng parameters", http.StatusBadRequest)
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		http.Error(w, "Invalid lat parameter", http.StatusBadRequest)
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		http.Error(w, "Invalid lng parameter", http.StatusBadRequest)
		return
	}

	areaName, areaId := findArea(lng, lat)

	if areaName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{
                        "results": [
                                {
                                        "address_components": [
                                                {
                                                        "long_name": "Dire Dawa",
                                                        "short_name": "Dire Dawa",
                                                        "types": ["locality", "political"]
                                                }
                                        ],
                                        "formatted_address": "Dire Dawa",
                                        "geometry": {
                                                "location": {
                                                        "lat": %f,
                                                        "lng": %f
                                                },
                                                "location_type": "APPROXIMATE"
                                        },
                                        "place_id": "unknown",
                                        "types": ["locality", "political"]
                                }
                        ],
                        "status": "OK"
                }`, lat, lng)))
		return
	}

	response := fmt.Sprintf(`{
                "results": [
                        {
                                "address_components": [
                                        {
                                                "long_name": "%s, Dire Dawa",
                                                "short_name": "%s",
                                                "types": ["locality", "political"]
                                        }
                                ],
                                "formatted_address": "%s",
                                "geometry": {
                                        "location": {
                                                "lat": %f,
                                                "lng": %f
                                        },
                                        "location_type": "APPROXIMATE"
                                },
                                "place_id": "%s",
                                "types": ["locality", "political"]
                        }
                ],
                "status": "OK"
        }`, areaName, areaName, areaName, lat, lng, areaId)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(response))
}

func findArea(lng float64, lat float64) (string, string) {
	data, err := ioutil.ReadFile("areas.json")
	if err != nil {
		log.Println("Error reading areas.json:", err)
		return "", ""
	}

	var featureCollection FeatureCollection
	err = json.Unmarshal(data, &featureCollection)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return "", ""
	}
	log.Printf("Number of features: %d", len(featureCollection.Features))
	for _, feature := range featureCollection.Features {
		// Corrected call: pass feature.Geometry.Coordinates[0][0]
		if isPointInPolygon(lng, lat, feature.Geometry.Coordinates[0]) {
			return feature.Properties.Name, feature.Properties.Id
		}
	}

	return "", ""
}

// Corrected function parameter type.
func isPointInPolygon(lng float64, lat float64, polygon [][]float64) bool {
	n := len(polygon)
	inside := false
	p1x, p1y := polygon[0][0], polygon[0][1]

	for i := 0; i < n+1; i++ {
		p2x, p2y := polygon[i%n][0], polygon[i%n][1]
		if lat > math.Min(p1y, p2y) {
			if lat <= math.Max(p1y, p2y) {
				if lng <= math.Max(p1x, p2x) {
					if p1y != p2y {
						xinters := (lat-p1y)*(p2x-p1x)/(p2y-p1y) + p1x
						if p1x == p2x || lng <= xinters {
							inside = !inside
						}
					}
				}
			}
		}
		p1x, p1y = p2x, p2y
	}
	return inside
}
