package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// PrecioLuz representa la estructura de la respuesta de la API de precios de luz
type PrecioLuz struct {
	Date       string  `json:"date"`
	Hour       string  `json:"hour"`
	IsCheap    bool    `json:"is-cheap"`
	IsUnderAvg bool    `json:"is-under-avg"`
	Market     string  `json:"market"`
	Price      float64 `json:"price"`
	Units      string  `json:"units"`
}

// PreciosLuz representa un mapa de horas a precios de luz
type PreciosLuz map[string]PrecioLuz

// ObtenerPreciosLuz hace una solicitud a la API de precios de luz y devuelve los precios
func ObtenerPreciosLuz(url string) (PreciosLuz, error) {
	log.Println("Solicitando datos de precios de luz desde la API")
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var precios PreciosLuz
	err = json.Unmarshal(body, &precios)
	if err != nil {
		return nil, err
	}

	log.Println("Datos de precios de luz obtenidos correctamente")
	return precios, nil
}

// EncenderEnchufe hace una solicitud para encender el enchufe
func EncenderEnchufe(url string) error {
	log.Println("Enviando solicitud para encender el enchufe")
	req, err := http.NewRequest("POST", url+"/encender", nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error al encender el enchufe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error al encender el enchufe: status code %d", resp.StatusCode)
	}

	log.Println("Enchufe encendido correctamente")
	return nil
}

// ApagarEnchufe hace una solicitud para apagar el enchufe
func ApagarEnchufe(url string) error {
	log.Println("Enviando solicitud para apagar el enchufe")
	req, err := http.NewRequest("POST", url+"/apagar", nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error al apagar el enchufe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error al apagar el enchufe: status code %d", resp.StatusCode)
	}

	log.Println("Enchufe apagado correctamente")
	return nil
}

// EncontrarRangoMasBarato encuentra el rango de 3 horas consecutivas más barato del día
func EncontrarRangoMasBarato(precios PreciosLuz) (horaInicio string, horaFin string) {
	var listaPrecios []PrecioLuz
	for _, precio := range precios {
		listaPrecios = append(listaPrecios, precio)
	}

	// Ordenar listaPrecios por la hora de inicio
	sort.Slice(listaPrecios, func(i, j int) bool {
		return listaPrecios[i].Hour < listaPrecios[j].Hour
	})

	minPrecio := float64(1<<63 - 1) // Un número muy grande
	var inicio int

	// Iterar sobre los precios para encontrar el rango de 3 horas consecutivas más barato
	for i := 0; i <= len(listaPrecios)-3; i++ {
		sumaPrecios := listaPrecios[i].Price + listaPrecios[i+1].Price + listaPrecios[i+2].Price
		if sumaPrecios < minPrecio {
			minPrecio = sumaPrecios
			inicio = i
		}
	}

	horaInicio = listaPrecios[inicio].Hour
	horaFin = listaPrecios[inicio+2].Hour
	return horaInicio, horaFin
}

// ConvierteHora convierte una hora en formato "hh-hh" a "15:04"
func ConvierteHora(hora string) string {
	partes := strings.Split(hora, "-")
	return partes[0] + ":00"
}

// ProgramarEncendido programa el encendido y apagado del enchufe
func ProgramarEncendido(horaInicio string, horaFin string, enchufeURL string) {
	horaInicio = ConvierteHora(horaInicio)
	
	// Obtener la fecha de hoy y combinarla con la hora de inicio
	now := time.Now()
	hoyInicio := fmt.Sprintf("%d-%02d-%02dT%s:00Z", now.Year(), now.Month(), now.Day(), horaInicio)
	horaInicioTime, err := time.Parse(time.RFC3339, hoyInicio)
	if err != nil {
		log.Println("Error al parsear la hora de inicio:", err)
		return
	}

	log.Printf("Hora de inicio: %s", horaInicioTime.Format("15:04"))

	// Verificar que la hora de inicio sea en el futuro
	if horaInicioTime.Before(now) {
		log.Println("La hora de inicio ya ha pasado, no se puede programar el encendido.")
		return
	}

	horaApagadoTime := horaInicioTime.Add(3 * time.Hour)

	log.Printf("Programando encendido del enchufe a las %s y apagado a las %s", horaInicioTime.Format("15:04"), horaApagadoTime.Format("15:04"))

	go func(encendido, apagado time.Time) {
		log.Println("Esperando hasta la hora de encendido")
		time.Sleep(time.Until(encendido))
		if err := EncenderEnchufe(enchufeURL); err != nil {
			log.Println(err)
			return
		}

		log.Println("Enchufe encendido")

		time.Sleep(3 * time.Hour)
		if err := ApagarEnchufe(enchufeURL); err != nil {
			log.Println(err)
		}
		log.Println("Enchufe apagado")
	}(horaInicioTime, horaApagadoTime)
}

func main() {
	// Cargar variables de entorno desde .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error cargando el archivo .env")
	}

	preciosLuzAPI := os.Getenv("PRECIOS_LUZ_API")
	enchufeAPI := os.Getenv("ENCHUFE_API")

	if preciosLuzAPI == "" || enchufeAPI == "" {
		log.Fatal("Las variables de entorno PRECIOS_LUZ_API y ENCHUFE_API deben estar definidas")
	}

	if !(startsWith(preciosLuzAPI, "http://") || startsWith(preciosLuzAPI, "https://")) {
		log.Fatal("PRECIOS_LUZ_API debe empezar con 'http://' o 'https://'")
	}
	if !(startsWith(enchufeAPI, "http://") || startsWith(enchufeAPI, "https://")) {
		log.Fatal("ENCHUFE_API debe empezar con 'http://' o 'https://'")
	}

	// Función para obtener precios y programar encendido
	actualizarYProgramar := func() {
		log.Println("Actualizando precios y programando encendido del enchufe")
		precios, err := ObtenerPreciosLuz(preciosLuzAPI)
		if err != nil {
			log.Println("Error al obtener los precios de luz:", err)
			return
		}

		horaInicio, horaFin := EncontrarRangoMasBarato(precios)
		ProgramarEncendido(horaInicio, horaFin, enchufeAPI)
	}

	// Actualizar y programar encendido al iniciar
	actualizarYProgramar()

	// Programar actualización diaria
	c := cron.New()
	c.AddFunc("@daily", actualizarYProgramar)
	c.Start()

	log.Println("El programa está en ejecución")

	// Para mantener el programa en ejecución
	select {}
}

// startsWith verifica si una cadena empieza con un prefijo dado
func startsWith(str, prefix string) bool {
	return len(str) >= len(prefix) && str[:len(prefix)] == prefix
}
