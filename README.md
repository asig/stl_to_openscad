# stl_to_openscad

This is a command line utility to convert STL files to OpenSCAD. The STL data
is embedded into a module, and optionally centered. The tool supports both 
binary and ASCII STL files, and can read from stdin and write to stdout for 
easy piping.

# How to build
`go build`

# Usage
Usage: 
`stl_to_openscad [-input <inputfile>] [-output <outputfile>] [-module <modulename>] [-center] [-single_polyhedron]`

## Flags

| Flag                 | Meaning                                                                                             |
|----------------------|-----------------------------------------------------------------------------------------------------|
| `-center`            | If true, the shape is centered in the (x,y) plane (default true).                                   |
| `-input`             | The input file to read from. If not set, the program reads from `stdin`.                            |
| `-output`            | The output file to write to. If not set, the program writes to `stdout`.                            |
| `-module`            | Name of the generated OpenSCAD module. Defaults to `shape`, or the solid name in an ASCII STL file. |
| `-single_polyhedron` | Whether a single polyhedron() call should be used, or one per facet. Defaults to `true`.            |
