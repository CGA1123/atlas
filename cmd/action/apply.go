package action

import (
	"context"
	"io/ioutil"
	"strings"

	"ariga.io/atlas/sql/schema"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	// ApplyFlags are the flags used in Apply command.
	ApplyFlags struct {
		DSN  string
		File string
		Web  bool
		Addr string
	}
	// ApplyCmd represents the apply command.
	ApplyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply an atlas schema to a target database.",
		Long: "`atlas schema apply`" + ` plans and executes a database migration to be bring a given database
to the state described in the Atlas schema file. Before running the migration, Atlas will print the migration
plan and prompt the user for approval.`,
		Run: CmdApplyRun,
		Example: `
atlas schema apply -d "mysql://user:pass@tcp(localhost:3306)/dbname" -f atlas.hcl
atlas schema apply -d "mariadb://user:pass@tcp(localhost:3306)/dbname" -f atlas.hcl
atlas schema apply --dsn "postgres://user:pass@host:port/dbname" -f atlas.hcl
atlas schema apply -d "sqlite://file:ex1.db?_fk=1" -f atlas.hcl`,
	}
)

const (
	answerApply = "Apply"
	answerAbort = "Abort"
)

func init() {
	schemaCmd.AddCommand(ApplyCmd)
	ApplyCmd.Flags().StringVarP(&ApplyFlags.DSN, "dsn", "d", "", "[driver://username:password@protocol(address)/dbname?param=value] Select data source using the dsn format")
	ApplyCmd.Flags().StringVarP(&ApplyFlags.File, "file", "f", "", "[/path/to/file] file containing schema")
	ApplyCmd.Flags().BoolVarP(&ApplyFlags.Web, "web", "w", false, "Open in a local Atlas UI")
	ApplyCmd.Flags().StringVarP(&ApplyFlags.Addr, "addr", "", "127.0.0.1:5800", "used with -w, local address to bind the server to")
	cobra.CheckErr(ApplyCmd.MarkFlagRequired("dsn"))
	cobra.CheckErr(ApplyCmd.MarkFlagRequired("file"))
}

// CmdApplyRun is the command used when running CLI.
func CmdApplyRun(cmd *cobra.Command, args []string) {
	if ApplyFlags.Web {
		schemaCmd.PrintErrln("The Atlas UI is not available in this release.")
		return
	}
	d, err := defaultMux.OpenAtlas(ApplyFlags.DSN)
	cobra.CheckErr(err)
	applyRun(d, ApplyFlags.DSN, ApplyFlags.File)
}

func applyRun(d *Driver, dsn string, file string) {
	ctx := context.Background()
	name, err := SchemaNameFromDSN(dsn)
	cobra.CheckErr(err)
	s, err := d.InspectSchema(ctx, name, nil)
	cobra.CheckErr(err)
	f, err := ioutil.ReadFile(file)
	cobra.CheckErr(err)
	var desired schema.Schema
	err = d.UnmarshalSpec(f, &desired)
	cobra.CheckErr(err)
	changes, err := d.SchemaDiff(s, &desired)
	cobra.CheckErr(err)
	if len(changes) == 0 {
		schemaCmd.Println("Schema is synced, no changes to be made")
		return
	}
	p, err := d.PlanChanges(ctx, "plan", changes)
	cobra.CheckErr(err)
	schemaCmd.Println("-- Planned Changes:")
	for _, c := range p.Changes {
		if c.Comment != "" {
			schemaCmd.Println("--", strings.ToUpper(c.Comment[:1])+c.Comment[1:])
		}
		schemaCmd.Println(c.Cmd)
	}
	prompt := promptui.Select{
		Label: "Are you sure?",
		Items: []string{answerApply, answerAbort},
	}
	_, result, err := prompt.Run()
	cobra.CheckErr(err)
	if result == answerApply {
		err = d.ApplyChanges(ctx, changes)
		cobra.CheckErr(err)
	}
}
