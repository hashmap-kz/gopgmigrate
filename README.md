### **Expected Migration File Formats**

#### **Versioned Migrations**

- **Format:** `#####-name.do.sql`
- **Requirements:**
    - Exactly **5 digits** (`#####`)
    - Followed by a **dash** (`-`)
    - Any **name** (`name`)
    - Ends with **`.do.sql`**

✅ **Examples:**

```
00003-users.do.sql
12345-init.do.sql
```

---

#### **Repeatable Migrations**

- **Format:** `*.r.sql`
- **Requirements:**
    - Any **file name** (`*`)
    - Ends with **`.r.sql`**

✅ **Examples:**

```
fn_get_users.r.sql
update_schema.r.sql
```

