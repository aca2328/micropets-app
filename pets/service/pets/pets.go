package pets

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/imroc/req"
	"github.com/opentracing/opentracing-go"

	otrext "github.com/opentracing/opentracing-go/ext"
	otrlog "github.com/opentracing/opentracing-go/log"

	. "moussaud.org/pets/internal"
)

var calls = 0

//Pet Structure
type Pet struct {
	Index    int
	Name     string
	Type     string
	Kind     string
	Age      int
	URL      string
	Hostname string
	From     string
	URI      string
}

//Path Structure
type Path struct {
	Service  string
	Hostname string
}

//Pets Structure
type Pets struct {
	Total     int
	Hostname  string
	Hostnames []Path
	Pets      []Pet `json:"Pets"`
}

func setupResponse(w *http.ResponseWriter, req *http.Request) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func lookupService(service string) string {

	fmt.Fprintf(os.Stderr, "-- Service %v\n", service)
	ips, err := net.LookupIP(service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
		return service
	}

	for _, ip := range ips {
		fmt.Printf("%s. IN A %s\n", service, ip.String())
	}

	return service
}

func queryPets(spanCtx opentracing.SpanContext, backend string) (Pets, error) {

	var pets Pets
	req.Debug = true
	fmt.Printf("##########################@ 2 Connecting backend [%s]\n", backend)
	req, err := http.NewRequest("GET", backend, nil)
	if err != nil {
		return pets, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Expires", "10ms")

	//Inject the opentracing header
	if LoadConfiguration().Observability.Enable {
		opentracing.GlobalTracer().Inject(spanCtx, opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("##########################@ ERROR Connecting backend [%s]\n", backend)
		return pets, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return pets, fmt.Errorf("ReadAll got error %s", err.Error())
	}

	json.Unmarshal(body, &pets)
	return pets, nil
}

func queryPet(spanCtx opentracing.SpanContext, backend string) (Pet, error) {

	var pet Pet
	req.Debug = true
	fmt.Printf("##########################@ 2 Connecting backend [%s]\n", backend)
	req, err := http.NewRequest("GET", backend, nil)
	if err != nil {
		return pet, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Expires", "10ms")

	//Inject the opentracing header
	if LoadConfiguration().Observability.Enable {
		opentracing.GlobalTracer().Inject(spanCtx, opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("##########################@ ERROR Connecting backend [%s]\n", backend)
		return pet, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return pet, fmt.Errorf("ReadAll got error %s", err.Error())
	}

	json.Unmarshal(body, &pet)
	return pet, nil
}

func readiness_and_liveness(w http.ResponseWriter, r *http.Request) {
	span := NewServerSpan(r, "readiness_and_liveness")
	defer span.Finish()

	setupResponse(&w, r)
	//fmt.Printf("Handling %+v\n", r)
	var all Pets
	path := Path{"pets", "readiness_and_liveness"}
	all.Hostnames = []Path{path}
	js, err := json.Marshal(all)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func pets(w http.ResponseWriter, r *http.Request) {
	span := NewServerSpan(r, "index")
	defer span.Finish()

	setupResponse(&w, r)
	fmt.Printf("index Handling %+v\n", r)
	config := LoadConfiguration()

	var all Pets
	host, err := os.Hostname()
	if err != nil {
		host = "Unknown"
	}
	path := Path{"pets", host}
	all.Hostnames = []Path{path}

	for i, backend := range config.Backends {
		URL := fmt.Sprintf("http://%s:%s%s", backend.Host, backend.Port, backend.Context)
		lookupService(backend.Host)
		fmt.Printf("* Accessing %d\t %s\t %s\n", i, backend.Name, URL)
		pets, err := queryPets(span.Context(), URL)
		if err != nil {
			fmt.Printf("* ERROR * Accessing backend [%s][%s]:[%s]\n", backend.Name, URL, err)
		} else {
			fmt.Printf("* process result\n")
			all.Total = all.Total + pets.Total
			all.Hostnames = append(all.Hostnames, Path{backend.Name, pets.Hostname})
			fmt.Printf("* Hostnames %s\n", all.Hostname)
			for _, pet := range pets.Pets {
				pet.Type = backend.Name
				pet.URI = fmt.Sprintf("/pets%s", pet.URI)
				all.Pets = append(all.Pets, pet)
			}
			time.Sleep(time.Duration(pets.Total) * time.Millisecond)
		}
	}

	sort.SliceStable(all.Pets, func(i, j int) bool {
		return all.Pets[i].Name < all.Pets[j].Name
	})

	calls = calls + 1
	if calls%50 == 0 {
		//fmt.Printf("Zero answer from all the services (0) %d\n ", calls)
		all.Total = 0
	}

	if all.Total == 0 {
		fmt.Printf("Zero answer from all the services (1)\n")
		otrext.Error.Set(span, true)
		span.LogFields(
			otrlog.String("error.kind", "global failure"),
			otrlog.String("message", "pet service unavailable"),
		)
		//http.Error(w, "Zero answer from all the services (1) ", http.StatusInternalServerError)
		WriteError(w, "no answer from all the pets services", http.StatusServiceUnavailable)
		return
	} else {
		js, err := json.Marshal(all)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}

func detail(w http.ResponseWriter, r *http.Request) {
	span := NewServerSpan(r, "detail")
	defer span.Finish()
	
	setupResponse(&w, r)
	fmt.Printf("index Handling %+v\n", r)
	config := LoadConfiguration()

	re := regexp.MustCompile(`/`)
	// /pets/dogs/v1/data/1
	submatchall := re.Split(r.URL.Path, -1)
	service := submatchall[2]
	id := submatchall[5]
	// TODO use the context provided by the request /pets/dogs/v1/data/1 => /dogs/v1/data/1

	//fmt.Fprintf(w, "Display a specific pet with ID %s ... => %s %s ", r.URL.Path, service, id)
	for _, backend := range config.Backends {
		if service == backend.Name {
			URL := fmt.Sprintf("http://%s:%s%s/%s", backend.Host, backend.Port, backend.Context, id)
			fmt.Printf("* Accessing %s\t %s\n", backend.Name, URL)
			pet, err := queryPet(span.Context(), URL)
			if err != nil {
				fmt.Printf("* ERROR * Accessing backend [%s][%s]:[%s]\n", backend.Name, URL, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {
				fmt.Printf("* process result\n")
				pet.Type = backend.Name
				js, err := json.Marshal(pet)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write(js)
			}
		}
	}

}

func Start() {

	config := LoadConfiguration()

	if config.Service.Listen {
		port := config.Service.Port
		http.HandleFunc("/readiness", readiness_and_liveness)
		http.HandleFunc("/liveness", readiness_and_liveness)
		http.HandleFunc("/pets", pets)
		http.HandleFunc("/pets/", detail)
		fmt.Printf("******* Starting to the Pets service on port %s\n", port)
		for i, backend := range config.Backends {
			fmt.Printf("* Managing %d\t %s\t %s:%s%s\n", i, backend.Name, backend.Host, backend.Port, backend.Context)
		}
		log.Fatal(http.ListenAndServe(port, nil))
	} else {
		fmt.Printf("******* Don't Execute Pets service and exit \n")
	}
}
