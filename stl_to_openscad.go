/*
 * stl_to_openscad: A command line utilitly to convert STL files to
 * OpenSCAD.
 *
 * Copyright (c) 2019 Andreas Signer <asigner@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

var (
	inputFilenameFlag  = flag.String("input", "", "The input file to read from. If not set, the program reads from stdin")
	outputFilenameFlag = flag.String("output", "", "The output file to write to. If not set, the program writes to stdout")
	moduleNameFlag     = flag.String("module", "", "Name of the generated OpenSCAD module")
	centerFlag         = flag.Bool("center", true, "If true, the shape is centered in the (x,y) plane")

	moduleName string
)

type point struct {
	x, y, z float32
}

func (p point) add(q point) point {
	return point{
		p.x + q.x,
		p.y + q.y,
		p.z + q.z,
	}
}

type polygon struct {
	vertices []point
}

type polygons []polygon

func (polygons polygons) boundingBox() (min, max point) {
	min = point{x: math.MaxFloat32, y: math.MaxFloat32, z: math.MaxFloat32}
	max = point{x: -math.MaxFloat32, y: -math.MaxFloat32, z: -math.MaxFloat32}
	for _, p := range polygons {
		for _, v := range p.vertices {
			min.x = float32(math.Min(float64(min.x), float64(v.x)))
			min.y = float32(math.Min(float64(min.y), float64(v.y)))
			min.z = float32(math.Min(float64(min.z), float64(v.z)))
			max.x = float32(math.Max(float64(max.x), float64(v.x)))
			max.y = float32(math.Max(float64(max.y), float64(v.y)))
			max.z = float32(math.Max(float64(max.z), float64(v.z)))
		}
	}
	return min, max
}

func expect(scanner *bufio.Scanner, expected string) {
	scanner.Scan()
	str := scanner.Text()
	if str != expected {
		log.Fatalf("Expected %q, but got %q.", expected, str)
	}
}

func readFloat(scanner *bufio.Scanner) float32 {
	var (
		val float64
		err error
	)
	scanner.Scan()
	s := scanner.Text()
	if val, err = strconv.ParseFloat(s, 32); err != nil {
		log.Fatalf("Can't convert %q to float: %s", s, err)
	}
	return float32(val)
}

func readPoint(scanner *bufio.Scanner) point {
	return point{
		x: readFloat(scanner),
		y: readFloat(scanner),
		z: readFloat(scanner),
	}
}

func readFacet(scanner *bufio.Scanner) polygon {
	poly := polygon{}

	// "facet" is already read
	expect(scanner, "normal")
	readPoint(scanner)
	expect(scanner, "outer")
	expect(scanner, "loop")

	expect(scanner, "vertex")
	poly.vertices = append(poly.vertices, readPoint(scanner))
	expect(scanner, "vertex")
	poly.vertices = append(poly.vertices, readPoint(scanner))
	expect(scanner, "vertex")
	poly.vertices = append(poly.vertices, readPoint(scanner))

	expect(scanner, "endloop")
	expect(scanner, "endfacet")

	return poly
}

func readAscii(r *bufio.Reader) polygons {
	var polygons polygons

	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	if err := scanner.Err(); err != nil {
		log.Fatal("Error parsing input: %s", err)
	}
	expect(scanner, "solid")
	scanner.Scan()
	shapeName := scanner.Text()
	if moduleName == "" {
		moduleName = shapeName
	}

	scanner.Scan()
	for scanner.Text() == "facet" {
		polygons = append(polygons, readFacet(scanner))
		scanner.Scan()
	}
	if scanner.Text() != "endsolid" {
		log.Fatalf("Expected %q, but got %q.", "endsolid", scanner.Text())
	}
	scanner.Scan() // Skip name

	return polygons
}

func readPointBinary(r *bufio.Reader) point {
	p := point{}
	binary.Read(r, binary.LittleEndian, &p.x)
	binary.Read(r, binary.LittleEndian, &p.y)
	binary.Read(r, binary.LittleEndian, &p.z)
	return p
}

func readBinary(r *bufio.Reader) polygons {
	var polygons polygons

	// Skip header
	r.Discard(80)

	var numTriangles uint32
	binary.Read(r, binary.LittleEndian, &numTriangles)

	for i := uint32(0); i < numTriangles; i++ {
		// ignore normal vector
		r.Discard(3 * 4)

		// read polygon
		p := polygon{}
		p.vertices = append(p.vertices, readPointBinary(r))
		p.vertices = append(p.vertices, readPointBinary(r))
		p.vertices = append(p.vertices, readPointBinary(r))
		polygons = append(polygons, p)

		// skip attribute byte count
		r.Discard(2)
	}

	return polygons
}

func openInput() io.Reader {
	var (
		r   io.Reader
		err error
	)
	if *inputFilenameFlag != "" {
		log.Printf("Reading from file %q", *inputFilenameFlag)
		if r, err = os.Open(*inputFilenameFlag); err != nil {
			log.Fatalf("Can't open file %q: %s", *inputFilenameFlag, err)
		}
		return r
	}
	log.Printf("Reading from stdin")
	return os.Stdin
}

func openOutput() io.Writer {
	var (
		w   io.Writer
		err error
	)
	if *outputFilenameFlag != "" {
		log.Printf("Writing to file %q", *outputFilenameFlag)
		if w, err = os.Create(*outputFilenameFlag); err != nil {
			log.Fatalf("Can't open file %q: %s", outputFilenameFlag, err)
		}
		return w
	}
	log.Printf("Writing to stdout")
	return os.Stdout
}

func ftos(val float32) string {
	s := fmt.Sprintf("%.10f", val)
	for strings.HasSuffix(s, "0") {
		s = s[0 : len(s)-1]
	}
	if strings.HasSuffix(s, ".") {
		s = s[0 : len(s)-1]
	}
	return s
}

func pointToOpenScad(pt point) string {
	return fmt.Sprintf("[%s,%s,%s]", ftos(pt.x), ftos(pt.y), ftos(pt.z))
}

func facesToOpenScad(start, len int) string {
	var f []string
	for i := 0; i < len; i++ {
		f = append(f, fmt.Sprintf("%d", start+i))
	}
	return "[" + strings.Join(f, ",") + "]"
}

func combineStrings(strings []string, count int, separator string) []string {
	var lines []string

	curLine := ""
	i := 0
	for idx, _ := range strings {
		part := strings[idx]
		if idx < len(strings)-1 {
			part = part + separator
		}
		if i < count {
			curLine = curLine + part
			i++
		} else {
			lines = append(lines, curLine)
			curLine = part
			i = 1
		}
	}
	lines = append(lines, curLine)
	return lines
}

func writeOpenScad(w *bufio.Writer, polygons polygons) {
	var (
		points []string
		faces  []string
	)
	ofs := 0
	for _, p := range polygons {
		for _, v := range p.vertices {
			points = append(points, pointToOpenScad(v))
		}
		faces = append(faces, facesToOpenScad(ofs, len(p.vertices)))
		ofs = ofs + len(p.vertices)
	}

	fmt.Fprintf(w, "module %s() {\n", moduleName)
	fmt.Fprintf(w, "  polyhedron(\n")
	fmt.Fprintf(w, "    points=[\n")
	for _, l := range combineStrings(points, 3, ", ") {
		fmt.Fprintf(w, "      %s\n", l)
	}
	fmt.Fprintf(w, "    ],\n")
	fmt.Fprintf(w, "    faces=[\n")
	for _, l := range combineStrings(faces, 6, ", ") {
		fmt.Fprintf(w, "      %s\n", l)
	}
	fmt.Fprintf(w, "    ]\n")
	fmt.Fprintf(w, "  );\n")
	fmt.Fprintf(w, "}\n")
	fmt.Fprintf(w, "%s();\n", moduleName)
	w.Flush()
}

func postProcess(polygons polygons) {
	if !*centerFlag {
		return
	}
	log.Print("Centering object")
	min, max := polygons.boundingBox()
	delta := point{
		-min.x - (max.x-min.x)/2,
		-min.y - (max.y-min.y)/2,
		-min.z,
	}
	for i, _ := range polygons {
		for j, _ := range polygons[i].vertices {
			polygons[i].vertices[j] = polygons[i].vertices[j].add(delta)
		}
	}
}

func init() {
	flag.Parse()
	moduleName = *moduleNameFlag
}

func main() {
	input := bufio.NewReader(openInput())
	output := bufio.NewWriter(openOutput())

	var polygons polygons
	// Read first few bytes to determine ascii or binary
	buf, _ := input.Peek(6)
	if string(buf) == "solid " {
		log.Print("Reading ASCII file")
		polygons = readAscii(input)
	} else {
		log.Print("Reading Binary file")
		polygons = readBinary(input)
	}
	log.Printf("# of facets: %d", len(polygons))

	if moduleName == "" {
		moduleName = "shape"
	}
	log.Printf("Using module name %q", moduleName)

	postProcess(polygons)

	writeOpenScad(output, polygons)
}
