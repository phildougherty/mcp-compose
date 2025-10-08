package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func NewWorkflowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage workflows",
		Long:  `List, delete, and manage workflows in the workflow builder.`,
	}

	cmd.AddCommand(NewWorkflowListCommand())
	cmd.AddCommand(NewWorkflowDeleteCommand())

	return cmd
}

func NewWorkflowListCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workflows",
		Long:  `List all workflows from the workflow builder.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dashboardURL := os.Getenv("DASHBOARD_URL")
			if dashboardURL == "" {
				dashboardURL = "http://localhost:3111"
			}

			url := fmt.Sprintf("%s/api/workflows", dashboardURL)
			if limit > 0 {
				url = fmt.Sprintf("%s?limit=%d", url, limit)
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to fetch workflows: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
			}

			var result struct {
				Count     int `json:"count"`
				Workflows []struct {
					ID        string `json:"id"`
					Name      string `json:"name"`
					Version   int    `json:"version"`
					CreatedAt string `json:"created_at"`
					UpdatedAt string `json:"updated_at"`
				} `json:"workflows"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			if result.Count == 0 {
				fmt.Println("No workflows found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tVERSION\tCREATED\tUPDATED")
			fmt.Fprintln(w, strings.Repeat("-", 36)+"\t"+strings.Repeat("-", 30)+"\t"+strings.Repeat("-", 7)+"\t"+strings.Repeat("-", 19)+"\t"+strings.Repeat("-", 19))

			for _, workflow := range result.Workflows {
				fmt.Fprintf(w, "%s\t%s\tv%d\t%s\t%s\n",
					workflow.ID,
					workflow.Name,
					workflow.Version,
					workflow.CreatedAt[:19],
					workflow.UpdatedAt[:19])
			}

			w.Flush()
			fmt.Printf("\nTotal: %d workflow(s)\n", result.Count)

			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "Maximum number of workflows to list")

	return cmd
}

func NewWorkflowDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [workflow-id]",
		Short: "Delete a workflow",
		Long:  `Delete a workflow by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowID := args[0]

			dashboardURL := os.Getenv("DASHBOARD_URL")
			if dashboardURL == "" {
				dashboardURL = "http://localhost:3111"
			}

			if !force {
				fmt.Printf("Are you sure you want to delete workflow %s? (y/N): ", workflowID)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			url := fmt.Sprintf("%s/api/workflows/%s", dashboardURL, workflowID)

			req, err := http.NewRequest(http.MethodDelete, url, nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to delete workflow: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
			}

			fmt.Printf("Successfully deleted workflow %s\n", workflowID)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func NewWorkflowDeleteAllCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete-all",
		Short: "Delete all workflows",
		Long:  `Delete all workflows from the database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dashboardURL := os.Getenv("DASHBOARD_URL")
			if dashboardURL == "" {
				dashboardURL = "http://localhost:3111"
			}

			resp, err := http.Get(fmt.Sprintf("%s/api/workflows", dashboardURL))
			if err != nil {
				return fmt.Errorf("failed to fetch workflows: %w", err)
			}
			defer resp.Body.Close()

			var result struct {
				Count     int `json:"count"`
				Workflows []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"workflows"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			if result.Count == 0 {
				fmt.Println("No workflows to delete")
				return nil
			}

			if !force {
				fmt.Printf("Are you sure you want to delete ALL %d workflow(s)? (y/N): ", result.Count)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			deleted := 0
			failed := 0

			for _, workflow := range result.Workflows {
				url := fmt.Sprintf("%s/api/workflows/%s", dashboardURL, workflow.ID)
				req, err := http.NewRequest(http.MethodDelete, url, nil)
				if err != nil {
					fmt.Printf("Failed to create request for %s: %v\n", workflow.Name, err)
					failed++
					continue
				}

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					fmt.Printf("Failed to delete %s: %v\n", workflow.Name, err)
					failed++
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					deleted++
					fmt.Printf("Deleted: %s\n", workflow.Name)
				} else {
					failed++
					fmt.Printf("Failed to delete %s (status %d)\n", workflow.Name, resp.StatusCode)
				}
			}

			fmt.Printf("\nDeleted: %d, Failed: %d\n", deleted, failed)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}
