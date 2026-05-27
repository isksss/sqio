package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

func newRolesCommand() *cobra.Command {
	var opts connectionOptions
	var format string
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "list database roles or users",
		RunE: func(cmd *cobra.Command, args []string) error {
			access, cleanup, err := accessService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			roles, err := access.Roles(ctx)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitConnection}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(roles)
			case "", "table":
				for _, role := range roles {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", role.Name, role.Host, loginLabel(role.Login))
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported roles format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	return cmd
}

func newGrantsCommand() *cobra.Command {
	var opts connectionOptions
	var format string
	var role string
	cmd := &cobra.Command{
		Use:   "grants",
		Short: "list database grants",
		RunE: func(cmd *cobra.Command, args []string) error {
			access, cleanup, err := accessService(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			grants, err := access.Grants(ctx, role)
			if err != nil {
				return &CommandError{Type: "metadata", Message: err.Error(), Code: ExitConnection}
			}
			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(grants)
			case "", "table":
				for _, grant := range grants {
					if grant.Raw != "" {
						fmt.Fprintln(cmd.OutOrStdout(), grant.Raw)
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", grant.Grantee, grant.Object, grant.Privilege, grantableLabel(grant.Grantable))
				}
				return nil
			default:
				return &CommandError{Type: "output", Message: "unsupported grants format: " + format, Code: ExitInternal}
			}
		},
	}
	addConnectionFlags(cmd.Flags(), &opts)
	cmd.Flags().StringVar(&role, "role", "", "filter grants by role")
	cmd.Flags().StringVar(&format, "format", "table", "output format")
	return cmd
}

func accessService(ctx context.Context, opts connectionOptions) (service.AccessService, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return service.AccessService{}, nil, err
	}
	driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts)
	if err != nil {
		return service.AccessService{}, nil, err
	}
	if driver == "" && dsn == "" {
		return service.AccessService{}, cleanup, &CommandError{Type: "connection", Message: "connection is required", Code: ExitConnection}
	}
	return service.AccessService{Driver: driver, DSN: dsn}, cleanup, nil
}

func loginLabel(login bool) string {
	if login {
		return "login"
	}
	return "no-login"
}

func grantableLabel(grantable bool) string {
	if grantable {
		return "grantable"
	}
	return "-"
}
