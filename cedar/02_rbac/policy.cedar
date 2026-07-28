// admin はすべての操作ができる。
permit (
    principal in Role::"admin",
    action,
    resource
);

// editor は読み取りと書き込みができる。
permit (
    principal in Role::"editor",
    action in [Action::"Read", Action::"Write"],
    resource
);

// viewer は読み取りだけができる。
permit (
    principal in Role::"viewer",
    action == Action::"Read",
    resource
);

// 本番リソースの削除は、admin であっても禁止する。
forbid (
    principal,
    action == Action::"Delete",
    resource
)
when { resource.environment == "production" };
