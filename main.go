package main

import (

	// Core
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	// Third party
	"github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/alecthomas/kingpin.v2"
	yaml "gopkg.in/yaml.v2"
)

var (

	// Create newi cli app instance.
	app = kingpin.New("app", "Codebase project management for pro's.")

	// $ proj init --name=MyProject --command="docker-compose build"
	initProject        = app.Command("init", "Create a new project.")
	initProjectName    = initProject.Flag("name", "Project name").Required().String()
	initProjectPath    = initProject.Flag("path", "Project path.").Required().String()
	initProjectCommand = initProject.Flag("command", "Boot command.").Required().String()

	// $ proj commit
	commit = app.Command("commit", "Commit a config file change.")

	// $ proj start my-project
	start     = app.Command("start", "Start your project.")
	startName = start.Arg("name", "Project name.").Required().String()
)

func cliError(err error) {
	color.Red(fmt.Sprintf("==> Error: %s\n", err.Error()))
	os.Exit(1)
}

// Proj - Main project instance.
type Proj struct {
	db *sql.DB
}

// NewProj - New instance of Proj app.
func NewProj(db *sql.DB) *Proj {
	return &Proj{db}
}

// Project - Project object
type Project struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Path    string `yaml:"path"`
	Command string `yaml:"command"`
}

// InitDB - Initialise database.
func InitDB(filepath string) *sql.DB {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		panic(err)
	}

	if db == nil {
		cliError(errors.New("DB Not found!"))
	}

	return db
}

// CreateTable - Create table if not exists.
func CreateTable(db *sql.DB) {
	table := `
        CREATE TABLE IF NOT EXISTS projects(
            Id TEXT NOT NULL PRIMARY KEY,
            Name TEXT,
            Path TEXT,
            Command TEXT,
            CreatedAt DATETIME DEFAULT CURRENT_TIMESTAMP
        );
    `

	_, err := db.Exec(table)
	if err != nil {
		cliError(errors.New("Failed to create database table."))
	}
}

// SaveProject - Save a project to the database.
func (proj *Proj) SaveProject(project Project) {
	sqlAdd := `
        INSERT OR REPLACE INTO projects(
            Id, 
            Name,
            Path,
            Command,
            CreatedAt
        ) values(?, ?, ?, ?, CURRENT_TIMESTAMP);
    `

	stmt, err := proj.db.Prepare(sqlAdd)

	defer stmt.Close()

	_, err = stmt.Exec(project.ID, project.Name, project.Path, project.Command)

	if err != nil {
		cliError(errors.New("Failed to save project."))
	}
}

// UpdateProject - Update a project in the database.
func (proj *Proj) UpdateProject(project Project) {
	update := `
        UPDATE projects
        SET Name = ?, Command = ?, Path = ?
        WHERE Id = ?
    `

	stmt, err := proj.db.Prepare(update)

	if err != nil {
		cliError(errors.New("Failed to update project."))
	}

	defer stmt.Close()

	_, err = stmt.Exec(project.Name, project.Command, project.Path, project.ID)

	if err != nil {
		log.Panic(err)
	}
}

// LoadProject - Load a project from the database.
func (proj *Proj) LoadProject(name string) Project {
	find := `
        SELECT Id, Name, Command, Path FROM projects
        WHERE Name = ?
    `

	row := proj.db.QueryRow(find, name)

	var project Project

	err := row.Scan(&project.ID, &project.Name, &project.Command, &project.Path)

	if err != nil {
		cliError(errors.New("Failed to load project."))
	}

	return project
}

func main() {

	const DbPath = "/tmp/projects.db"

	db := InitDB(DbPath)
	defer db.Close()
	CreateTable(db)

	proj := NewProj(db)

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case initProject.FullCommand():
		fmt.Println(*initProjectName)
		project := Project{
			ID:      "123",
			Name:    *initProjectName,
			Path:    *initProjectPath,
			Command: *initProjectCommand,
		}
		proj.InitProject(project)

	case commit.FullCommand():
		fmt.Println("Updating...")
		proj.CommitChanges()

	case start.FullCommand():
		fmt.Println("Starting " + *startName)
		proj.StartProject(*startName)
	}
}

// InitProject - Create new project.
func (proj *Proj) InitProject(project Project) {

	// Create a YAML file from project details.
	proj.CreateProjectFile(project)
	proj.SaveProject(project)
}

// StartProject - Start a project.
func (proj *Proj) StartProject(name string) {

	// Load project
	project := proj.LoadProject(name)

	// Run start command
	cmd := exec.Command("sh", "-c", project.Command, project.Path)

	// Stdout buffer
	cmdOutput := &bytes.Buffer{}

	// Attach buffer to command
	cmd.Stdout = cmdOutput

	// Execute command
	printCommand(cmd)

	err := cmd.Run() // will wait for command to return

	if err != nil {
		cliError(err)
	}

	// Only output the commands stdout
	printOutput(cmdOutput.Bytes())
}

func printCommand(cmd *exec.Cmd) {
	color.Magenta("==> Executing: %s\n", strings.Join(cmd.Args, " "))
}

func printOutput(outs []byte) {
	if len(outs) > 0 {
		color.Blue("==> Output: %s\n", string(outs))
	}
}

// CreateProjectFile - Create a project file.
func (proj *Proj) CreateProjectFile(project Project) {

	// Save a yaml file
	data, err := yaml.Marshal(&project)

	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(project.Path+"/proj.yml", data, 0755)

	if err != nil {
		panic(err)
	}
}

// CommitChanges - Commit file changes to the database.
func (proj *Proj) CommitChanges() {

	var project Project

	// Load yaml file
	data, err := ioutil.ReadFile("./proj.yml")

	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, &project)

	if err != nil {
		panic(err)
	}

	proj.UpdateProject(project)
}
