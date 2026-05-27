package cli

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/isksss/sqio/internal/service"
	"github.com/spf13/cobra"
)

type dumpOptions struct {
	connectionOptions
	table   string
	format  string
	out     string
	gzip    bool
	maxRows int
}

type loadOptions struct {
	connectionOptions
	table  string
	format string
	file   string
	gzip   bool
}

func newDumpCommand() *cobra.Command {
	var opts dumpOptions
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "dump table rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.table == "" {
				return &CommandError{Type: "input", Message: "--table is required", Code: ExitInternal}
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			if driver == "" && dsn == "" {
				return &CommandError{Type: "connection", Message: "connection is required", Code: ExitConnection}
			}
			w, closeOut, err := tableOutputTarget(cmd.OutOrStdout(), opts.out, opts.gzip)
			if err != nil {
				return err
			}
			defer closeOut()
			_, err = service.DumpTable(ctx, w, service.DumpOptions{
				Driver: driver, DSN: dsn, Table: opts.table, Format: opts.format, MaxRows: opts.maxRows,
			})
			if err != nil {
				return &CommandError{Type: "output", Message: err.Error(), Code: ExitInternal}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.table, "table", "", "table name")
	cmd.Flags().StringVar(&opts.format, "format", "csv", "output format")
	cmd.Flags().StringVar(&opts.out, "out", "", "output file")
	cmd.Flags().BoolVar(&opts.gzip, "gzip", false, "gzip-compress output")
	cmd.Flags().IntVar(&opts.maxRows, "max-rows", 0, "maximum rows")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func newLoadCommand() *cobra.Command {
	var opts loadOptions
	cmd := &cobra.Command{
		Use:   "load",
		Short: "load table rows",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.table == "" {
				return &CommandError{Type: "input", Message: "--table is required", Code: ExitInternal}
			}
			if opts.file == "" {
				return &CommandError{Type: "input", Message: "--file is required", Code: ExitInternal}
			}
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			driver, dsn, cleanup, err := prepareConnection(ctx, cfg, opts.connectionOptions)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			if driver == "" && dsn == "" {
				return &CommandError{Type: "connection", Message: "connection is required", Code: ExitConnection}
			}
			input, closeIn, err := tableInput(opts.file, opts.gzip)
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
			}
			defer closeIn()
			result, err := service.LoadTable(ctx, input, service.LoadOptions{Driver: driver, DSN: dsn, Table: opts.table, Format: opts.format})
			if err != nil {
				return &CommandError{Type: "input", Message: err.Error(), Code: ExitInternal}
			}
			if !global.quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "OK (%d rows)\n", result.RowsAffected)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.table, "table", "", "table name")
	cmd.Flags().StringVar(&opts.file, "file", "", "input file")
	cmd.Flags().StringVar(&opts.format, "format", "csv", "input format")
	cmd.Flags().BoolVar(&opts.gzip, "gzip", false, "gzip-decompress input")
	addConnectionFlags(cmd.Flags(), &opts.connectionOptions)
	return cmd
}

func tableOutputTarget(defaultWriter io.Writer, path string, useGzip bool) (io.Writer, func(), error) {
	w, closeOut, err := outputTarget(defaultWriter, path)
	if err != nil {
		return nil, nil, err
	}
	if !useGzip && !strings.HasSuffix(path, ".gz") {
		return w, closeOut, nil
	}
	gw := gzip.NewWriter(w)
	return gw, func() {
		_ = gw.Close()
		closeOut()
	}, nil
}

func tableInput(path string, useGzip bool) (io.Reader, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	closeIn := func() { _ = file.Close() }
	if !useGzip && !strings.HasSuffix(path, ".gz") {
		return file, closeIn, nil
	}
	gr, err := gzip.NewReader(file)
	if err != nil {
		closeIn()
		return nil, nil, err
	}
	return gr, func() {
		_ = gr.Close()
		closeIn()
	}, nil
}
