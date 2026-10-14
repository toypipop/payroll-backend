package models

import "sort"

// PermissionAuditData snapshots only grants/scopes, never the associated models.
func PermissionAuditData(rows []RolesModuleAccess) map[string]any {
	ordered := append([]RolesModuleAccess(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ModuleID < ordered[j].ModuleID })
	grants := make([]map[string]any, 0, len(ordered))
	for _, row := range ordered {
		grants = append(grants, map[string]any{"module_id": row.ModuleID, "can_view": row.CanView,
			"can_create": row.CanCreate, "can_edit": row.CanEdit, "can_delete": row.CanDelete, "data_scope": row.DataScope})
	}
	return map[string]any{"permission_count": len(rows), "permissions": grants}
}
