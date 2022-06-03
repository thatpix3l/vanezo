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

package cmd

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/thatpix3l/fntwo/app"
	"github.com/thatpix3l/fntwo/cfg"
)

var (
	// Home of config and data files.
	// Neat and tidy according to freedesktop.org's base directory specifications.
	// Along with whatever Windows does, I guess...

	appName          = "fntwo"                  // Name of program. Duh...
	envPrefix        = strings.ToUpper(appName) // Prefix for all environment variables used for configuration
	initCfgNameNoExt = "config"                 // Name of the default config file used, without an extension

	initCfgDir       = path.Join(xdg.ConfigHome, appName)      // Default path to config directory
	initCfgFileNoExt = path.Join(initCfgDir, initCfgNameNoExt) // Default path to config file, without extension
	sceneDataDir     = path.Join(xdg.DataHome, appName)        // Default path to scene-related data directory
	sceneCfgFile     = path.Join(sceneDataDir, "scene.json")   // Default path to scene config file, like camera state
	vrmFile          = path.Join(sceneDataDir, "default.vrm")  // Default path to VRM file that will be loaded and overwritten
)

// Entrypoint for command line
func Start() {

	// Create scene config home for data
	os.MkdirAll(sceneDataDir, 0644)

	// Build out root command
	cmd := newRootCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// Take a command, create env variables that are mapped to most flags, load config
func initializeConfig(cmd *cobra.Command) {

	cmdFlags := cmd.Flags()

	// Viper config that will be merged from different file sources and env variables
	v := viper.New()

	// Setting properties of the config file, before reading and processing
	v.SetConfigName(initCfgNameNoExt) // Default config name, without extension
	v.AddConfigPath(initCfgDir)       // Path to search for config files
	v.SetEnvPrefix(envPrefix)         // Prefix for all environment variables
	v.AutomaticEnv()                  // Auto-check if any config keys match env keys

	// If config flag was manually set by the user, set that as the config file to be loaded
	cfgFlag := cmd.Flag("config")
	if cfgFlag.Changed {
		log.Print("Default config file was changed")
		v.SetConfigFile(cfgFlag.Value.String())
	}

	// Read in config file
	if err := v.ReadInConfig(); err != nil {
		log.Print(err)
	}

	// Create equivalent env var keys for each flag, replace value in flag if not
	// explicitly changed by the user on the command line
	cmdFlags.VisitAll(func(f *pflag.Flag) {

		// Config is a special case. We only want it to be configurable from the command line
		if f.Name == "config" {
			return
		}

		// Create an env var key mapped to a flag, e.g. "FOO_BAR" is created from "foo-bar".
		// Take same env var key name, and normalize it to env var naming specification, e.g. "FOO_BAR",
		// so when assigning FOO_BAR=baz, it maps to foo-bar
		envKey := envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		v.BindEnv(f.Name, envKey)

		// If command flag is not set and equivalent config key is set,
		// assign to flag the config value
		if !f.Changed && v.IsSet(f.Name) {
			flagVal := v.Get(f.Name)
			cmdFlags.Set(f.Name, fmt.Sprintf("%v", flagVal))
		}

	})

}

// Loads and parses config files from different sources,
// parses them, and finally merges them together
func newRootCommand() *cobra.Command {

	// Config var for app initialization
	var initCfg cfg.Initial

	// Base command of actual program
	rootCmd := &cobra.Command{
		Use:   appName,
		Short: `Function Two`,
		Long:  `An easy to use tool for loading, configuring and displaying your VTuber models`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			// Load and merge config from different sources, based on command flags
			initializeConfig(cmd)

		},
		Run: func(cmd *cobra.Command, args []string) {

			// Entrypoint for actual program
			app.Start(&initCfg)

		},
	}

	// Here, we start defining a load of flags
	rootFlags := rootCmd.Flags()
	rootFlags.StringVarP(&initCfg.InitialCfgFile, "config", "c", initCfgFileNoExt+".{json,yaml,toml,ini}", "Path to a config file.")
	rootFlags.StringVar(&initCfg.VmcListenIP, "vmc-ip", "0.0.0.0", "Address to listen and receive on for VMC motion data")
	rootFlags.IntVar(&initCfg.VmcListenPort, "vmc-port", 39540, "Port to listen and receive on for VMC motion data")
	rootFlags.StringVar(&initCfg.WebServeIP, "web-ip", "127.0.0.1", "Address to serve frontend page on")
	rootFlags.IntVar(&initCfg.WebServePort, "web-port", 3579, "Port to serve frontend page on")
	rootFlags.IntVar(&initCfg.ModelUpdateFrequency, "update-frequency", 60, "Times per second the live VRM model data is sent to each client")
	rootFlags.StringVar(&initCfg.SceneCfgFile, "scene-cfg", sceneCfgFile, "Path to config file for storing and retrieving scene data, like camera state")
	rootFlags.StringVar(&initCfg.VRMFile, "vrm-file", vrmFile, "Path to VRM file to load on startup and overwrite")

	return rootCmd

}
