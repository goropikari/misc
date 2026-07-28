// 所有者は自分のドキュメントを閲覧・編集できる。
permit (
    principal,
    action in [Action::"View", Action::"Edit"],
    resource
)
when { principal == resource.owner };

// readers は閲覧だけできる。
permit (
    principal,
    action == Action::"View",
    resource
)
when { principal in resource.readers };

// 機密ドキュメントは所有者以外の編集を明示的に拒否する。
forbid (
    principal,
    action == Action::"Edit",
    resource
)
when {
    resource.classification == "confidential" &&
    principal != resource.owner
};
