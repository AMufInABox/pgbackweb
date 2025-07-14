package databases

import (
	"database/sql"
	"net/http"

	"github.com/eduardolat/pgbackweb/internal/service/databases"
	"github.com/eduardolat/pgbackweb/internal/validate"
	"github.com/eduardolat/pgbackweb/internal/view/web/component"
	"github.com/eduardolat/pgbackweb/internal/view/web/respondhtmx"
	"github.com/labstack/echo/v4"
	nodx "github.com/nodxdev/nodxgo"
	htmx "github.com/nodxdev/nodxgo-htmx"
	lucide "github.com/nodxdev/nodxgo-lucide"
)

type discoverDatabasesDTO struct {
	Version          string `form:"version" validate:"required"`
	ConnectionString string `form:"connection_string" validate:"required"`
}

type bulkImportDTO struct {
	Version           string   `form:"version" validate:"required"`
	ConnectionString  string   `form:"connection_string" validate:"required"`
	SelectedDatabases []string `form:"selected_databases"`
}

func (h *handlers) discoverDatabasesHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData discoverDatabasesDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	discovered, err := h.servs.DatabasesService.DiscoverDatabases(
		ctx, formData.Version, formData.ConnectionString,
	)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	return c.HTML(http.StatusOK, databasesTable(discovered, formData.Version, formData.ConnectionString))
}

func (h *handlers) bulkImportDatabasesHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData bulkImportDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	if len(formData.SelectedDatabases) == 0 {
		return respondhtmx.ToastError(c, "Please select at least one database to import")
	}

	req := databases.BulkImportRequest{
		ConnectionString:  formData.ConnectionString,
		Version:          formData.Version,
		SelectedDatabases: formData.SelectedDatabases,
	}

	err := h.servs.DatabasesService.BulkImportDatabases(ctx, req)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	return respondhtmx.Redirect(c, "/dashboard/databases")
}

func databasesTable(databases []databases.DatabaseDiscovery, version, connectionString string) string {
	if len(databases) == 0 {
		return `<div class="text-center py-8">
			<p class="text-gray-500">No databases found</p>
		</div>`
	}

	rows := ""
	for _, db := range databases {
		rows += `
			<tr>
				<td>
					<input type="checkbox" name="selected_databases" value="` + db.Name + `" class="checkbox checkbox-primary">
				</td>
				<td class="font-medium">` + db.Name + `</td>
				<td>` + db.Size + `</td>
				<td>` + db.Owner + `</td>
				<td>` + db.Comment + `</td>
			</tr>
		`
	}

	return `
		<div class="space-y-4">
			<div class="flex justify-between items-center">
				<h3 class="text-lg font-medium">Discovered Databases</h3>
				<div class="flex items-center space-x-2">
					<button type="button" onclick="checkAll()" class="btn btn-sm btn-outline">Select All</button>
					<button type="button" onclick="uncheckAll()" class="btn btn-sm btn-outline">Select None</button>
				</div>
			</div>
			<div class="overflow-x-auto">
				<table class="table w-full">
					<thead>
						<tr>
							<th class="w-1"></th>
							<th>Name</th>
							<th>Size</th>
							<th>Owner</th>
							<th>Comment</th>
						</tr>
					</thead>
					<tbody>
						` + rows + `
					</tbody>
				</table>
			</div>
			<input type="hidden" name="version" value="` + version + `">
			<input type="hidden" name="connection_string" value="` + connectionString + `">
			<div class="flex justify-end">
				<button type="submit" class="btn btn-primary">
					<span>Import Selected Databases</span>
					<svg class="w-4 h-4 ml-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"></path>
					</svg>
				</button>
			</div>
		</div>
		<script>
			function checkAll() {
				const checkboxes = document.querySelectorAll('input[name="selected_databases"]');
				checkboxes.forEach(cb => cb.checked = true);
			}
			function uncheckAll() {
				const checkboxes = document.querySelectorAll('input[name="selected_databases"]');
				checkboxes.forEach(cb => cb.checked = false);
			}
		</script>
	`
}

func bulkImportDatabaseButton() nodx.Node {
	htmxAttributes := func(url string) nodx.Node {
		return nodx.Group(
			htmx.HxPost(url),
			htmx.HxInclude("#bulk-import-form"),
			htmx.HxDisabledELT(".bulk-import-btn"),
			htmx.HxIndicator("#bulk-import-loading"),
			htmx.HxValidate("true"),
		)
	}

	mo := component.Modal(component.ModalParams{
		Size:  component.SizeLg,
		Title: "Bulk Import Databases",
		Content: []nodx.Node{
			nodx.FormEl(
				nodx.Id("bulk-import-form"),
				nodx.Class("space-y-4"),
				htmx.HxPost("/dashboard/databases/bulk-import"),

				nodx.Div(
					nodx.Class("space-y-2"),
					component.SelectControl(component.SelectControlParams{
						Name:        "version",
						Label:       "PostgreSQL Version",
						Placeholder: "Select a version",
						Required:    true,
						HelpText:    "The version of PostgreSQL server to connect to",
						Children: []nodx.Node{
							component.PGVersionSelectOptions(sql.NullString{}),
						},
					}),

					component.InputControl(component.InputControlParams{
						Name:        "connection_string",
						Label:       "Connection String",
						Placeholder: "postgresql://user:password@localhost:5432/",
						Required:    true,
						Type:        component.InputTypeText,
						HelpText:    "Base connection string without specific database name. We'll query all databases from this server.",
					}),
				),

				nodx.Div(
					nodx.Class("flex justify-between items-center pt-4"),
					nodx.Button(
						htmxAttributes("/dashboard/databases/bulk-import/discover"),
						htmx.HxTarget("#discovered-databases"),
						nodx.Class("bulk-import-btn btn btn-outline"),
						nodx.Type("button"),
						component.SpanText("Discover Databases"),
						lucide.Search(),
					),
					component.HxLoadingMd("bulk-import-loading"),
				),

				nodx.Div(
					nodx.Id("discovered-databases"),
					nodx.Class("mt-4"),
				),
			),
		},
	})

	button := nodx.Button(
		mo.OpenerAttr,
		nodx.Class("btn btn-secondary"),
		component.SpanText("Bulk Import"),
		lucide.Database(),
	)

	return nodx.Div(
		nodx.Class("inline-block"),
		mo.HTML,
		button,
	)
}
