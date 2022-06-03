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

package main

import "github.com/thatpix3l/fntwo/cmd"

// Precompilation steps

//go:generate echo "Compiling frontend..."
//go:generate npm --prefix frontend/static run build

//go:generate echo "Pulling version information..."
//go:generate go run generate_version/generate_version.go

//go:generate echo "Compiling final binary..."
//go:generate go build -o build

// The actual program begins in cmd.Start
func main() {
	cmd.Start()
}
