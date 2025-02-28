package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/neo4j/neo4j-go-driver/neo4j"
)

// Estructuras para modelar los datos
type BusStop struct {
	Name string `json:"name"`
}

type Connection struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Distance float64 `json:"distance"`
}

type GraphData struct {
	BusStops    []BusStop    `json:"busStops"`
	Connections []Connection `json:"connections"`
}

func main() {
	// Configura el driver de Neo4j
	uri := "bolt://localhost:7687"
	// Cambia "your_password" por la contraseña de tu instancia Neo4j
	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth("neo4j", "your_password", ""))
	if err != nil {
		log.Fatal("Error al crear el driver de Neo4j: ", err)
	}
	defer driver.Close()

	// Crea datos de ejemplo en Neo4j
	session, err := driver.Session(neo4j.AccessModeWrite)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	// Limpia la base de datos para comenzar de cero
	_, err = session.Run("MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		log.Fatal("Error limpiando la base de datos: ", err)
	}

	// Define consultas para crear nodos (paradas de bus) y relaciones (conexiones)
	queries := []string{
		"CREATE (a:BusStop {name: 'A'})",
		"CREATE (b:BusStop {name: 'B'})",
		"CREATE (c:BusStop {name: 'C'})",
		"MATCH (a:BusStop {name: 'A'}), (b:BusStop {name: 'B'}) CREATE (a)-[:CONNECTED_TO {distance: 10}]->(b)",
		"MATCH (b:BusStop {name: 'B'}), (c:BusStop {name: 'C'}) CREATE (b)-[:CONNECTED_TO {distance: 15}]->(c)",
		"MATCH (a:BusStop {name: 'A'}), (c:BusStop {name: 'C'}) CREATE (a)-[:CONNECTED_TO {distance: 25}]->(c)",
	}

	for _, q := range queries {
		_, err = session.Run(q, nil)
		if err != nil {
			log.Fatal("Error ejecutando la consulta: ", err)
		}
	}

	// Configura el endpoint que devuelve los datos del grafo en formato JSON
	http.HandleFunc("/graph", func(w http.ResponseWriter, r *http.Request) {
		graphData, err := getGraphData(driver)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(graphData)
	})

	// Sirve una página HTML para la visualización
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, htmlPage)
	})

	log.Println("Servidor corriendo en http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// getGraphData consulta Neo4j para extraer nodos y relaciones y empaqueta los datos en GraphData
func getGraphData(driver neo4j.Driver) (*GraphData, error) {
	session, err := driver.Session(neo4j.AccessModeRead)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// Consulta para obtener todas las paradas de bus
	busStopsResult, err := session.Run("MATCH (n:BusStop) RETURN n.name AS name", nil)
	if err != nil {
		return nil, err
	}
	var busStops []BusStop
	for busStopsResult.Next() {
		record := busStopsResult.Record()
		name, _ := record.Get("name")
		busStops = append(busStops, BusStop{Name: name.(string)})
	}
	if err = busStopsResult.Err(); err != nil {
		return nil, err
	}

	// Consulta para obtener todas las conexiones entre paradas
	connectionsResult, err := session.Run("MATCH (a:BusStop)-[r:CONNECTED_TO]->(b:BusStop) RETURN a.name AS from, b.name AS to, r.distance AS distance", nil)
	if err != nil {
		return nil, err
	}
	var connections []Connection
	for connectionsResult.Next() {
		record := connectionsResult.Record()
		from, _ := record.Get("from")
		to, _ := record.Get("to")
		distance, _ := record.Get("distance")
		connections = append(connections, Connection{
			From:     from.(string),
			To:       to.(string),
			Distance: distance.(float64),
		})
	}
	if err = connectionsResult.Err(); err != nil {
		return nil, err
	}

	return &GraphData{
		BusStops:    busStops,
		Connections: connections,
	}, nil
}

// htmlPage contiene una página HTML básica que utiliza vis-network para visualizar el grafo
var htmlPage = `
<!DOCTYPE html>
<html>
<head>
  <title>Visualización del Grafo de Rutas de Buses</title>
  <script type="text/javascript" src="https://unpkg.com/vis-network/standalone/umd/vis-network.min.js"></script>
  <style type="text/css">
    #mynetwork {
      width: 800px;
      height: 600px;
      border: 1px solid lightgray;
    }
  </style>
</head>
<body>
  <h2>Visualización del Grafo</h2>
  <div id="mynetwork"></div>
  <script type="text/javascript">
    // Se obtiene la información del grafo desde el endpoint /graph
    fetch('/graph')
      .then(response => response.json())
      .then(data => {
        // Se preparan los nodos a partir de las paradas
        var nodes = new vis.DataSet(data.busStops.map(function(busStop) {
          return {id: busStop.name, label: busStop.name};
        }));
        // Se preparan las aristas a partir de las conexiones
        var edges = new vis.DataSet(data.connections.map(function(conn) {
          return {from: conn.from, to: conn.to, label: conn.distance.toString()};
        }));
        var container = document.getElementById('mynetwork');
        var networkData = { nodes: nodes, edges: edges };
        var options = {
          layout: { hierarchical: false },
          edges: { arrows: 'to' }
        };
        // Se crea la visualización del grafo
        var network = new vis.Network(container, networkData, options);
      })
      .catch(err => console.error(err));
  </script>
</body>
</html>
`
