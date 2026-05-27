package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "migrate", Short: "database migration operations"}
	cmd.AddCommand(newMigrateStatusCommand(), newMigratePlanCommand(), newMigrateApplyCommand(), newMigrateRollbackCommand(), newMigrateBaselineCommand())
	return cmd
}

func newMigrateStatusCommand() *cobra.Command {
	var opts connectionOptions
	var dir string
	var format string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrations, cleanup, err := migrationService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			status, err := migrations.Status(ctx, dir)
			if err != nil {
				return &CommandError{Type: "migration", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			case "", "table":
				for _, migration := range status {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", migration.Version, appliedLabel(migration.Applied), migration.Name)
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported migration format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&dir, "dir", "migrations", "migration directory")
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	return cmd
}

func newMigrateApplyCommand() *cobra.Command {
	var opts connectionOptions
	var dir string
	var format string
	var limit int
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrations, cleanup, err := migrationService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			result, err := migrations.Apply(ctx, dir, limit)
			if err != nil {
				return &CommandError{Type: "migration", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			case "", "table":
				for _, migration := range result.Applied {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\tapplied\t%s\n", migration.Version, migration.Name)
				}
				if len(result.Applied) == 0 && !global.quiet {
					fmt.Fprintln(cmd.OutOrStdout(), "up-to-date")
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported migration format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&dir, "dir", "migrations", "migration directory")
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum migrations to apply")
	return cmd
}

func newMigratePlanCommand() *cobra.Command {
	var opts connectionOptions
	var dir string
	var format string
	var rollbackLimit int
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "show migration apply and rollback plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrations, cleanup, err := migrationService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			plan, err := migrations.Plan(ctx, dir, rollbackLimit)
			if err != nil {
				return &CommandError{Type: "migration", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(plan)
			case "", "table":
				for _, migration := range plan.Pending {
					fmt.Fprintf(cmd.OutOrStdout(), "apply\t%s\t%s\n", migration.Version, migration.Name)
				}
				for _, migration := range plan.Rollback {
					fmt.Fprintf(cmd.OutOrStdout(), "rollback\t%s\t%s\n", migration.Version, migration.Name)
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported migration format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&dir, "dir", "migrations", "migration directory")
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().IntVar(&rollbackLimit, "rollback-limit", 1, "maximum rollback candidates")
	return cmd
}

func newMigrateRollbackCommand() *cobra.Command {
	var opts connectionOptions
	var dir string
	var format string
	var limit int
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "rollback applied migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrations, cleanup, err := migrationService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			result, err := migrations.Rollback(ctx, dir, limit)
			if err != nil {
				return &CommandError{Type: "migration", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			case "", "table":
				for _, migration := range result.Applied {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\trolled-back\t%s\n", migration.Version, migration.Name)
				}
				if len(result.Applied) == 0 && !global.quiet {
					fmt.Fprintln(cmd.OutOrStdout(), "up-to-date")
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported migration format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&dir, "dir", "migrations", "migration directory")
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().IntVar(&limit, "limit", 1, "maximum migrations to roll back")
	return cmd
}

func newMigrateBaselineCommand() *cobra.Command {
	var opts connectionOptions
	var dir string
	var format string
	var version string
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "record migrations as already applied",
		RunE: func(cmd *cobra.Command, args []string) error {
			migrations, cleanup, err := migrationService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			result, err := migrations.Baseline(ctx, dir, version)
			if err != nil {
				return &CommandError{Type: "migration", Message: err.Error(), Code: ExitInternal}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			case "", "table":
				for _, migration := range result.Applied {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\tbaseline\t%s\n", migration.Version, migration.Name)
				}
				if len(result.Applied) == 0 && !global.quiet {
					fmt.Fprintln(cmd.OutOrStdout(), "up-to-date")
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported migration format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&dir, "dir", "migrations", "migration directory")
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	cmd.Flags().StringVar(&version, "version", "", "baseline through version")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func migrationService(ctx context.Context, opts connectionOptions) (service.MigrationService, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return service.MigrationService{}, nil, err
	}
	driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts)
	if err != nil {
		return service.MigrationService{}, nil, err
	}
	if driver == "" && dsn == "" {
		return service.MigrationService{}, cleanup, &CommandError{Type: "connection", Message: "connection is required", Code: ExitConnection}
	}
	return service.MigrationService{Driver: driver, DSN: dsn}, cleanup, nil
}

func appliedLabel(applied bool) string {
	if applied {
		return "applied"
	}
	return "pending"
}
