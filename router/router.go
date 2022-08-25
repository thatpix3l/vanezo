/*
fntwo: An easy to use tool for VTubing
Copyright (C) 2022 thatpix3l <contact@thatpix3l.com>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package router

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/thatpix3l/fntwo/config"
	"github.com/thatpix3l/fntwo/frontend"
	"github.com/thatpix3l/fntwo/helper"
	"github.com/thatpix3l/fntwo/obj"
	"github.com/thatpix3l/fntwo/pool"
	"github.com/thatpix3l/fntwo/receivers"
)

type receiverInfo struct {
	Active    string   `json:"active"`
	Available []string `json:"available"`
}

// Helper func to allow all origin, headers, and methods for HTTP requests.
func allowHTTPAllPerms(wPtr *http.ResponseWriter) {

	// Set CORS policy
	(*wPtr).Header().Set("Access-Control-Allow-Origin", "*")
	(*wPtr).Header().Set("Access-Control-Allow-Methods", "*")
	(*wPtr).Header().Set("Access-Control-Allow-Headers", "*")

}

// Save a given scene to the default path
func saveScene(scene *config.Scene, sceneFilePath string) error {

	// Convert the scene config in memory into bytes
	sceneCfgBytes, err := json.MarshalIndent(scene, "", " ")
	if err != nil {
		return err
	}

	// Store config bytes into file
	if err := os.WriteFile(sceneFilePath, sceneCfgBytes, 0644); err != nil {
		return err
	}

	return nil

}

func New(appConfig *config.App, sceneConfig *config.Scene, receiverMap map[string]*receivers.MotionReceiver) *mux.Router {

	// Use picked receiver from user
	if receiverMap[appConfig.Receiver] == nil {
		log.Printf("Suggested receiver \"%s\" does not exist!", appConfig.Receiver)
	}

	activeReceiver := receiverMap[appConfig.Receiver]

	// Router for API and web frontend
	router := mux.NewRouter()

	// Route for relaying the internal state of the camera to all clients
	cameraPool := pool.New()
	router.HandleFunc("/live/read/camera", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Adding new camera client...")

		// Upgrade GET request to WebSocket
		ws, err := helper.WebSocketUpgrade(w, r)
		if err != nil {
			log.Println(err)
		}

		// On first-time connect, send the camera state
		if err := ws.WriteJSON(sceneConfig.Camera); err != nil {
			log.Println(err)
			return
		}

		// Add a new camera client
		cameraPool.Create(func(relayedData interface{}, client *pool.Client) {

			var ok bool // Boolean for if the pool's relayedData was type asserted as obj.Camera
			if sceneConfig.Camera, ok = relayedData.(obj.Camera); !ok {
				log.Println("Couldn't type assert relayed data as a camera")
				return
			}

			// Write camera data to connected frontend client
			if err := ws.WriteJSON(sceneConfig.Camera); err != nil {
				log.Println(err)
				client.Delete()
				ws.Close()
			}

		})

	})

	router.HandleFunc("/live/write/camera", func(w http.ResponseWriter, r *http.Request) {

		ws, err := helper.WebSocketUpgrade(w, r)
		if err != nil {
			log.Println(err)
			return
		}

		for {

			if err := ws.ReadJSON(&sceneConfig.Camera); err != nil {
				return
			}

			cameraPool.Update(sceneConfig.Camera)

		}

	})

	// Route for updating VRM model data to all clients
	router.HandleFunc("/live/read/model", func(w http.ResponseWriter, r *http.Request) {

		// Upgrade model data client into a WebSocket
		ws, err := helper.WebSocketUpgrade(w, r)
		if err != nil {
			log.Println(err)
			return
		}

		if err := ws.WriteJSON(activeReceiver.VRM); err != nil {
			log.Println(err)
			return
		}

		for {

			// Process and send the VRM data to WebSocket
			activeReceiver.VRM.Read(func(vrm *obj.VRM) {

				// Send VRM data to WebSocket client
				if err := ws.WriteJSON(*vrm); err != nil {
					return
				}

			})

			// Wait for whatever how long, per second. By default, 1/60 of a second
			time.Sleep(time.Duration(1e9 / appConfig.ModelUpdateFrequency))

		}

	})

	// Route for getting the default VRM model
	router.HandleFunc("/api/model", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to retrieve default VRM file")

		// Set model name and CORS policy
		w.Header().Set("Content-Disposition", "attachment; filename=default.vrm")
		allowHTTPAllPerms(&w)

		// Serve default VRM file
		http.ServeFile(w, r, appConfig.VRMFilePath)

	}).Methods("GET", "OPTIONS")

	// Route for setting the default VRM model
	router.HandleFunc("/api/model/update", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to set default VRM file")

		allowHTTPAllPerms(&w)

		// Destination VRM file on system
		dest, err := os.Create(appConfig.VRMFilePath)
		if err != nil {
			log.Println(err)
			return
		}

		// Copy request body binary to destination on system
		if _, err := io.Copy(dest, r.Body); err != nil {
			log.Println(err)
			return
		}

	}).Methods("PUT", "OPTIONS")

	// Route for saving the internal state of the scene config
	router.HandleFunc("/api/config/scene/update", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to save current scene")

		// Access control
		allowHTTPAllPerms(&w)

		if err := saveScene(sceneConfig, appConfig.SceneFilePath); err != nil {
			log.Println(err)
			return
		}

	}).Methods("PUT", "OPTIONS")

	router.HandleFunc("/api/config/scene", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to retrieve current state of scene config")

		allowHTTPAllPerms(&w)

		sceneConfigBytes, err := json.Marshal(sceneConfig)
		if err != nil {
			log.Println(err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(sceneConfigBytes)

	})

	// Route for retrieving the initial config for the server
	router.HandleFunc("/api/config/app", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to retrieve initial config")

		// Access control
		allowHTTPAllPerms(&w)

		// Marshal initial config into bytes
		appConfigBytes, err := json.Marshal(appConfig)
		if err != nil {
			log.Println(err)
			return
		}

		// Reply back to request with byte-format of initial config
		w.Header().Set("Content-Type", "application/json")
		w.Write(appConfigBytes)

	}).Methods("GET", "OPTIONS")

	// Route for retrieving info about receivers, including which one is in use
	router.HandleFunc("/api/receiver", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to list all available motion receivers")

		var receiverNames []string
		for name := range receiverMap {
			receiverNames = append(receiverNames, name)
		}

		info := receiverInfo{
			Active:    appConfig.Receiver,
			Available: receiverNames,
		}

		bytes, err := json.Marshal(info)
		if err != nil {
			log.Println(err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)

	}).Methods("GET", "OPTIONS")

	// Route for changing the active MotionReceiver source used
	router.HandleFunc("/api/receiver/update", func(w http.ResponseWriter, r *http.Request) {

		log.Println("Received request to change the MotionReceiver source for model")

		// Read in the request body into bytes, cast to string
		reqBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			return
		}
		suggestedReceiver := string(reqBytes)

		// Error if the suggested receiver does not exist
		if receiverMap[suggestedReceiver] == nil {
			log.Printf("\"%s\" does not exist!", suggestedReceiver)
			return
		}

		// Stop the current receiver
		activeReceiver.Stop()

		// Switch the active receiver
		appConfig.Receiver = suggestedReceiver
		activeReceiver = receiverMap[appConfig.Receiver]

		// Start the new receiver
		activeReceiver.Start()

		log.Printf("Successfully changed the active receiver to %s", suggestedReceiver)

	}).Methods("PUT", "OPTIONS")

	// All other requests are sent to the embedded web frontend
	frontendRoot, err := frontend.FS()
	if err != nil {
		log.Fatal(err)
	}
	router.PathPrefix("/").Handler(http.FileServer(http.FS(frontendRoot)))

	return router

}
