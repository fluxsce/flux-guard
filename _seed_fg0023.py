import sqlite3

con = sqlite3.connect("data/flux_guard.db", timeout=5)
cur = con.cursor()
exists = cur.execute(
    "SELECT count(*) FROM FG_AUTH_RESOURCE WHERE tenantId = 'default' AND resourceId = 'fg0023'"
).fetchone()[0]
if exists:
    print("already")
else:
    cur.execute("BEGIN")
    cur.executemany(
        """
        INSERT INTO FG_AUTH_RESOURCE (
          resourceId, tenantId, resourceName, resourceCode, resourceType,
          resourcePath, parentResourceId, resourceLevel, sortOrder, iconClass, language,
          resourceStatus, builtInFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
        ) VALUES (?, 'default', ?, ?, ?, ?, ?, ?, ?, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y')
        """,
        [
            ("fg0023", "资源", "fg0023", "MODULE", "/resources", "group:system", 2, 80),
            ("fg0023:view", "查看", "fg0023:view", "BUTTON", None, "fg0023", 3, 10),
        ],
    )
    grants = [
        ("RR_SA_fg0023", "ROLE_SUPER_ADMIN", "fg0023"),
        ("RR_SA_fg0023_view", "ROLE_SUPER_ADMIN", "fg0023:view"),
        ("RR_VW_fg0023", "ROLE_VIEWER", "fg0023"),
        ("RR_VW_fg0023_view", "ROLE_VIEWER", "fg0023:view"),
    ]
    cur.executemany(
        """
        INSERT INTO FG_AUTH_ROLE_RESOURCE (
          roleResourceId, tenantId, roleId, resourceId, permissionType, grantedBy,
          addWho, editWho, oprSeqFlag, currentVersion, activeFlag
        ) VALUES (?, 'default', ?, ?, 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y')
        """,
        grants,
    )
    con.commit()
    print("inserted")
names = cur.execute(
    "SELECT resourceId, resourceName, resourcePath FROM FG_AUTH_RESOURCE WHERE resourceId LIKE 'fg0023%'"
).fetchall()
open("_idcheck.txt", "w", encoding="utf-8").write("\n".join(f"{a}\t{b}\t{c}" for a, b, c in names))
