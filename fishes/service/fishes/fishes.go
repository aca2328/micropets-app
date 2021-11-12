package fishes

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "moussaud.org/fishes/internal"
)

//Fish Struct
type Fish struct {
	Name string
	Kind string
	Age  int
	URL  string
}

//fishes Struct
type Fishes struct {
	Total    int
	Hostname string
	Fishes   []Fish `json:"Pets"`
}

var calls = 0

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func index(w http.ResponseWriter, r *http.Request) {
	span := NewServerSpan(r, "index")
	defer span.Finish()

	setupResponse(&w, r)
	host, err := os.Hostname()
	if err != nil {
		host = "Unknown"
	}

	fishes := Fishes{4,
		host,
		[]Fish{
			{"Nemo", "Poisson Clown", 14,
				"https://www.sciencesetavenir.fr/assets/img/2019/07/10/cover-r4x3w1000-5d258790dd324-f96f05d4901fc6ce0ab038a685e4d5c99f6cdfe2-jpg.jpg"},
			{"Glumpy", "Neon Tetra", 11,
				"https://www.fishkeepingworld.com/wp-content/uploads/2018/02/Neon-Tetra-New.jpg"},
			{"Dory", "Pacific regal blue tang", 12,
				"http://www.oceanlight.com/stock-photo/palette-surgeonfish-image-07922-671143.jpg"},
			{"Argo", "Combattant", 27,
				"https://www.aquaportail.com/pictures1003/anemone-clown_1267799900_poisson-combattant.jpg"}}}

	calls = calls + 1

	if GlobalConfig.Service.Mode == "RANDOM_NUMBER" {
		total := rand.Intn(fishes.Total) + 1
		fmt.Printf("total %d\n", total)
		for i := 1; i < total; i++ {
			fishes.Fishes = fishes.Fishes[:len(fishes.Fishes)-1]
			fishes.Total = fishes.Total - 1
		}
	}

	js, err := json.Marshal(fishes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if GlobalConfig.Service.Delay.Period > 0 {
		y := float64(calls) * math.Pi / float64(2*GlobalConfig.Service.Delay.Period)
		sin_y := math.Sin(y)
		abs_y := math.Abs(sin_y)
		sleep := int(abs_y * GlobalConfig.Service.Delay.Amplitude * 1000.0)
		fmt.Printf("waitTime %d - %f - %f - %f  -> sleep %d seconds  \n", calls, y, math.Sin(y), abs_y, sleep)
		start := time.Now()
		time.Sleep(time.Duration(sleep) * time.Millisecond)
		elapsed := time.Since(start)
		fmt.Printf("Current Unix Time: %s\n", elapsed)
	}

	if GlobalConfig.Service.FrequencyError > 0 && calls%GlobalConfig.Service.FrequencyError == 0 {
		fmt.Printf("Fails this call (%d)", calls)
		http.Error(w, "Unexpected Error when querying the fishes repository", http.StatusServiceUnavailable)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}

//GetLocation returns the full path of the config file based on the current executable location or using SERVICE_CONFIG_DIR env
func GetLocation(file string) string {
	if serviceConfigDirectory := os.Getenv("SERVICE_CONFIG_DIR"); serviceConfigDirectory != "" {
		fmt.Printf("Load configuration from %s\n", serviceConfigDirectory)
		return filepath.Join(serviceConfigDirectory, file)
	} else {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath := filepath.Dir(ex)
		return filepath.Join(exPath, file)
	}
}

func readiness_and_liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

func Start() {
	config := LoadConfiguration()

	http.HandleFunc("/fishes/v1/data", index)

	http.HandleFunc("/fishes/liveness", readiness_and_liveness)
	http.HandleFunc("/fishes/readiness", readiness_and_liveness)

	http.HandleFunc("/liveness", readiness_and_liveness)
	http.HandleFunc("/readiness", readiness_and_liveness)

	//http.HandleFunc("/", fallback)

	fmt.Printf("******* Starting to the fishes service on port %s, mode %s\n", config.Service.Port, config.Service.Mode)
	fmt.Printf("******* Delay Period %d Amplitude %f\n", config.Service.Delay.Period, config.Service.Delay.Amplitude)
	fmt.Printf("******* Frequency Error %d\n", config.Service.FrequencyError)
	log.Fatal(http.ListenAndServe(config.Service.Port, nil))
}
