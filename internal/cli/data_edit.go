package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

type editOptions struct {
	connectionOptions
	table string
	set   []string
	where string
}

func newEditCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "edit", Short: "edit table rows"}
	cmd.AddCommand(newEditInsertCommand(), newEditUpdateCommand(), newEditDeleteCommand())
	return cmd
}

func newEditInsertCommand() *cobra.Command {
	var opts editOptions
	cmd := &cobra.Command{
		Use:   "insert",
		Short: "insert one table row",
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := parseSetValues(opts.set)
			if err != nil {
				return err
			}
			editor, cleanup, err := dataEditService(cmd.Context(), opts.connectionOptions)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			affected, err := editor.Insert(ctx, opts.table, values)
			if err != nil {
				return &CommandError{Type: "edit", Message: err.Error(), Code: ExitInternal}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK (%d rows)\n", affected)
			return nil
		},
	}
	addEditFlags(cmd, &opts)
	return cmd
}

func newEditUpdateCommand() *cobra.Command {
	var opts editOptions
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update table rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := parseSetValues(opts.set)
			if err != nil {
				return err
			}
			editor, cleanup, err := dataEditService(cmd.Context(), opts.connectionOptions)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			affected, err := editor.Update(ctx, opts.table, values, opts.where)
			if err != nil {
				return &CommandError{Type: "edit", Message: err.Error(), Code: ExitInternal}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK (%d rows)\n", affected)
			return nil
		},
	}
	addEditFlags(cmd, &opts)
	cmd.Flags().StringVar(&opts.where, "where", "", "SQL WHERE clause without the WHERE keyword")
	return cmd
}

func newEditDeleteCommand() *cobra.Command {
	var opts editOptions
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "delete table rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor, cleanup, err := dataEditService(cmd.Context(), opts.connectionOptions)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			affected, err := editor.Delete(ctx, opts.table, opts.where)
			if err != nil {
				return &CommandError{Type: "edit", Message: err.Error(), Code: ExitInternal}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK (%d rows)\n", affected)
			return nil
		},
	}
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	cmd.Flags().StringVar(&opts.table, "table", "", "table name")
	cmd.Flags().StringVar(&opts.where, "where", "", "SQL WHERE clause without the WHERE keyword")
	return cmd
}

func addEditFlags(cmd *cobra.Command, opts *editOptions) {
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	cmd.Flags().StringVar(&opts.table, "table", "", "table name")
	cmd.Flags().StringArrayVar(&opts.set, "set", nil, "column=value assignment")
}

func parseSetValues(assignments []string) (map[string]string, error) {
	values := map[string]string{}
	for _, assignment := range assignments {
		column, value, ok := strings.Cut(assignment, "=")
		column = strings.TrimSpace(column)
		if !ok || column == "" {
			return nil, &CommandError{Type: "input", Message: "set values must use column=value", Code: ExitInternal}
		}
		values[column] = value
	}
	if len(values) == 0 {
		return nil, &CommandError{Type: "input", Message: "at least one --set value is required", Code: ExitInternal}
	}
	return values, nil
}

func dataEditService(ctx context.Context, opts connectionOptions) (service.DataEditService, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return service.DataEditService{}, nil, err
	}
	driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts)
	if err != nil {
		return service.DataEditService{}, nil, err
	}
	if driver == "" && dsn == "" {
		return service.DataEditService{}, cleanup, &CommandError{Type: "connection", Message: "connection is required", Code: ExitConnection}
	}
	return service.DataEditService{Driver: driver, DSN: dsn}, cleanup, nil
}
