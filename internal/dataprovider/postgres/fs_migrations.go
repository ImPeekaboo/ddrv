package postgres

var fsTable = `
CREATE TABLE IF NOT EXISTS fs
(
    id     UUID PRIMARY KEY      DEFAULT gen_random_uuid(),
    name   VARCHAR(255) NOT NULL,
    dir    BOOL         NOT NULL DEFAULT FALSE,
    atime  TIMESTAMP    NOT NULL DEFAULT NOW(),
    mtime  TIMESTAMP    NOT NULL DEFAULT NOW(),
    parent UUID REFERENCES fs (id) ON DELETE CASCADE,
    UNIQUE (name, parent) -- This means that in the same directory, two files or directories cannot have the same name.
);
`

var nodeTable = `
CREATE TABLE node
(
    id    UUID PRIMARY KEY      DEFAULT gen_random_uuid(),
    file  UUID         NOT NULL REFERENCES fs (id) ON DELETE CASCADE,
    url   VARCHAR(255) NOT NULL,
    size  INTEGER      NOT NULL,
    iv    VARCHAR(255) NOT NULL DEFAULT '',
    mtime TIMESTAMP    NOT NULL DEFAULT NOW()
);
`

var fsParentIdx = `CREATE INDEX idx_fs_parent ON fs (parent);`
var fsNameIdx = `CREATE INDEX idx_fs_name ON fs (name);`

var rootInsert = `
INSERT INTO fs (id, name, dir, parent)
VALUES ('11111111-1111-1111-1111-111111111111', '', TRUE, NULL);
`

var statFunction = `
CREATE OR REPLACE FUNCTION stat(filepath TEXT)
    RETURNS TABLE
            (
                ID     UUID,
                NAME   TEXT,
                DIR    BOOL,
                SIZE   BIGINT,
                ATIME  TIMESTAMP,
                MTIME  TIMESTAMP,
                PARENT UUID
            )
AS
$$
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'stat');

    RETURN QUERY
        WITH RECURSIVE vfs
                           AS
                           (SELECT *, fs.name::TEXT AS path
                            FROM fs
                            WHERE fs.parent IS NULL
                            UNION ALL
                            SELECT f.*, p.path || '/' || f.name AS path
                            FROM fs f
                                     JOIN vfs p ON f.parent = p.id)
        SELECT vfs.id,
               parseroot(vfs.path)       AS name,
               vfs.dir,
               parsesize(SUM(node.size)) AS size,
               vfs.atime,
               vfs.mtime,
               vfs.parent
        FROM vfs
                 LEFT JOIN node ON node.file = vfs.id
        WHERE parseroot(vfs.path) = filepath
        GROUP BY 1, 2, 3, 5, 6, 7;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION stat IS 'This function returns the metadata of the file or directory specified by the given file path.';
`

var lsFunction = `
CREATE OR REPLACE FUNCTION ls(filepath TEXT)
    RETURNS TABLE
            (
                ID     UUID,
                NAME   TEXT,
                DIR    BOOL,
                SIZE   BIGINT,
                ATIME  TIMESTAMP,
                MTIME  TIMESTAMP,
                PARENT UUID
            )
AS
$$
DECLARE
    _id  UUID;
    _dir BOOL;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'ls');

    SELECT s.id, s.dir
    FROM stat(filepath) AS s
    INTO _id, _dir;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'ls % no such file or directory', filepath USING ERRCODE = 'P0002';
    END IF;

    IF _dir = FALSE THEN
        RAISE EXCEPTION 'ls % not a directory', filepath USING ERRCODE = 'P0004';
    END IF;

    IF filepath = '/' THEN
        filepath = '';
    END IF;
    RETURN QUERY
        WITH RECURSIVE vfs
                           AS
                           (SELECT *, filepath AS path
                            FROM fs
                            WHERE fs.id = _id
                            UNION ALL
                            SELECT f.*, p.path || '/' || f.name AS path
                            FROM fs f
                                     JOIN vfs p ON f.parent = p.id AND p.id = _id)
        SELECT vfs.id,
               parseroot(vfs.path)       AS name,
               vfs.dir,
               parsesize(SUM(node.size)) AS size,
               vfs.atime,
               vfs.mtime,
               vfs.parent
        FROM vfs
                 LEFT JOIN node ON node.file = vfs.id
        WHERE vfs.path != filepath
        GROUP BY 1, 2, 3, 5, 6, 7;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION ls IS 'The ls function lists the contents of a directory specified by the file path.';
`

var treeFunction = `
CREATE OR REPLACE FUNCTION tree(filepath TEXT)
    RETURNS TABLE
            (
                ID     UUID,
                NAME   TEXT,
                DIR    BOOL,
                SIZE   BIGINT,
                ATIME  TIMESTAMP,
                MTIME  TIMESTAMP,
                PARENT UUID
            )
AS
$$
DECLARE
    _id UUID;
    _dir BOOL;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'tree');

    SELECT s.id, s.dir
    FROM stat(filepath) AS s
    INTO _id, _dir;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'tree % no such file or directory', filepath USING ERRCODE = 'P0002';
    END IF;

    IF _dir = FALSE THEN
        RAISE EXCEPTION 'ls % not a directory', filepath USING ERRCODE = 'P0004';
    END IF;

    IF filepath = '/' THEN
        filepath = '';
    END IF;
    RETURN QUERY
        WITH RECURSIVE vfs
                           AS
                           (SELECT *, filepath AS path
                            FROM fs
                            WHERE fs.id = _id
                            UNION ALL
                            SELECT f.*, p.path || '/' || f.name AS path
                            FROM fs f
                                     JOIN vfs p ON f.parent = p.id)
        SELECT vfs.id,
               parseroot(vfs.path)       AS name,
               vfs.dir,
               parsesize(SUM(node.size)) AS size,
               vfs.atime,
               vfs.mtime,
               vfs.parent
        FROM vfs
                 LEFT JOIN node ON node.file = vfs.id
        WHERE vfs.path != filepath
        GROUP BY 1, 2, 3, 5, 6, 7;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION tree IS 'The tree function returns all files and directories under the specified directory recursively.';
`

var touchFunction = `
CREATE OR REPLACE FUNCTION touch(filepath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _id     UUID;
    _eid    UUID;
    _dir    BOOL;
    fname   TEXT;
    dirpath TEXT;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'touch');
    dirpath := dirname(filepath);
    fname := basename(filepath);
    PERFORM validfname(fname::TEXT);

    SELECT s.id, s.dir
    FROM stat(dirpath) AS s
    INTO _id, _dir;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'touch % no such file or directory', dirpath USING ERRCODE = 'P0002';
    END IF;
    IF _dir = FALSE THEN
        RAISE EXCEPTION 'touch % not a directory', dirpath USING ERRCODE = 'P0004';
    END IF;

    SELECT ID FROM fs WHERE parent = _id AND name = fname INTO _eid;
    IF _eid IS NOT NULL THEN
        DELETE FROM node WHERE file = _eid;
    ELSE
        INSERT INTO fs (name, dir, parent) VALUES (fname, FALSE, _id);
    END IF;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION touch IS 'This function is used to create a new file.';
`

var mkdirFunction = `
CREATE OR REPLACE FUNCTION mkdir(filepath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _id        UUID;
    _parent_id UUID;
    _path      TEXT[] := STRING_TO_ARRAY(sanitizefpath(filepath, FALSE, 'mkdir'), '/');
    _name      TEXT;
    _dir       BOOL;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, FALSE, 'mkdir');

    -- Iterates over each part of the path
    FOR i IN 1..ARRAY_LENGTH(_path, 1)
        LOOP
            _name := _path[i];

            -- Tries to find the current part of the path in the parent directory
            SELECT id, dir
            INTO _id, _dir
            FROM fs
            WHERE name = _name
              AND (i = 1 OR parent = _parent_id);

            IF _dir = FALSE THEN
                RAISE EXCEPTION 'mkdir % no such file or directory', filepath USING ERRCODE = 'P0002';
            END IF;
            -- If the directory doesn't exist, create it
            IF _id IS NULL THEN
                INSERT INTO fs (name, dir, parent) VALUES (_name, TRUE, _parent_id) RETURNING id INTO _id;
            END IF;

            -- Sets the current directory as the parent for the next loop
            _parent_id := _id;
        END LOOP;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION mkdir IS 'This function creates a new directory recursively. Equivalent to mkdir -p';
`

var mvFunction = `
CREATE OR REPLACE FUNCTION mv(oldpath TEXT, newpath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _old_id          UUID;
    _new_parent_id   UUID;
    _new_name        TEXT;
    _new_path        TEXT[] := STRING_TO_ARRAY(sanitizefpath(newpath, FALSE, 'mv'), '/');
    _new_parent_path TEXT;
BEGIN
    --- sanitize inputs
    oldpath = sanitizefpath(oldpath, FALSE, 'mv');
    newpath = sanitizefpath(newpath, FALSE, 'mv');

    -- If old path doesn't exist, raise an error
    SELECT s.id
    FROM stat(oldpath) AS s
    INTO _old_id;

    IF _old_id IS NULL THEN
        RAISE EXCEPTION 'mv % no such file or directory', oldpath USING ERRCODE = 'P0002';
    END IF;

    -- Split newpath into parent path and name
    _new_name := _new_path[ARRAY_LENGTH(_new_path, 1)];

    -- Construct the parent path manually
    _new_parent_path := '';
    FOR i IN 1..(ARRAY_LENGTH(_new_path, 1) - 1)
        LOOP
            _new_parent_path := _new_parent_path || '/' || _new_path[i];
        END LOOP;

    -- If new parent path doesn't exist, raise an error
    SELECT s.id
    FROM stat(_new_parent_path) AS s
    INTO _new_parent_id;

    IF _new_parent_id IS NULL THEN
        RAISE EXCEPTION 'mv % no such file or directory', newpath USING ERRCODE = 'P0002';
    END IF;

    -- Update the parent id and name of the old path
    UPDATE fs SET parent = _new_parent_id, name = _new_name WHERE id = _old_id;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION mv IS 'The mv function is used to move or rename files or directories.';
`

var rmFunction = `
CREATE OR REPLACE FUNCTION rm(filepath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _id UUID;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, FALSE, 'rm');

    SELECT s.id
    FROM stat(filepath) AS s
    INTO _id;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'rm % no such file or directory',filepath USING ERRCODE = 'P0002';
    END IF;

    DELETE FROM fs WHERE id = _id;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION rm IS 'The rm function is used to delete a file or directory recursively, Equivalent to rm -rf';
`

var resetFunction = `
CREATE OR REPLACE FUNCTION reset()
    RETURNS VOID
AS
$$
BEGIN
    DELETE FROM fs WHERE parent IS NOT NULL;
END;
$$ LANGUAGE plpgsql;
COMMENT ON FUNCTION rm IS 'This function deletes all files and directories except the root.';
`

var parserootFunction = `
CREATE OR REPLACE FUNCTION parseroot(filepath TEXT)
    RETURNS TEXT
AS
$$
BEGIN
    IF filepath = '' THEN
        RETURN '/';
    ELSE
        RETURN filepath;
    END IF;
END;
$$ LANGUAGE plpgsql;
`

var validnameFunction = `
CREATE OR REPLACE FUNCTION validfname(filename TEXT)
    RETURNS VOID
AS
$$
BEGIN
    -- first, check if the filename is not NULL or an empty string.
    -- next, check if it does not start with a space.
    -- then, check if the filename doesn't contain any invalid characters (like /, <, >, :, ", |, ?, or *).
    -- finally, if all conditions are met, return true; otherwise, return false.
    IF filename IS NOT NULL AND filename != '' AND filename !~ '^ ' AND filename !~ '[/<>"\|\?\*]' THEN
    ELSE
        RAISE EXCEPTION 'invalid filename %', filename USING ERRCODE = 'P0002';
    END IF;
END;
$$ LANGUAGE plpgsql;
`

var sanitizeFPath = `
CREATE OR REPLACE FUNCTION sanitizefpath(filepath TEXT, root BOOL, op TEXT)
    RETURNS TEXT
AS
$$
BEGIN
    -- merge slashes /home//darsh -> /home/darsh
    filepath = REGEXP_REPLACE(filepath, '/+', '/', 'g');
    -- root path is not allowed for few operations
    IF root = FALSE AND filepath = '/' THEN
        RAISE EXCEPTION 'operation % not allowed on root directory', op USING ERRCODE = 'P0001';
    END IF;
    -- remove last slash from path, /data/d1/ to /data/d1
    IF filepath <> '/' AND filepath ~ '/$' THEN
        filepath = SUBSTRING(filepath, 1, LENGTH(filepath) - 1);
    END IF;
    -- first, check if the filepath is not NULL or an empty string.
    -- next, ensure it starts with a slash (/).
    -- then, check if it doesn't contain any null characters or any segment starting with a space.
    -- finally, if all conditions are met, return true; otherwise, return false.
    IF filepath IS NOT NULL AND filepath != '' AND filepath ~ '^\/' AND filepath !~ '[\0]' AND
       filepath !~ '(^|/) [^/]*' THEN
    ELSE
        RAISE EXCEPTION 'invalid filepath %', filepath USING ERRCODE = 'P0002';
    END IF;

    RETURN filepath;
END;
$$ LANGUAGE plpgsql;
`

var parseSizeFunction = `
CREATE OR REPLACE FUNCTION parsesize(size BIGINT)
    RETURNS BIGINT
AS
$$
BEGIN
    IF size IS NULL THEN
        RETURN 0;
    END IF;
    RETURN size;
END;
$$
    LANGUAGE plpgsql;
`

var dirnameFunction = `

CREATE OR REPLACE FUNCTION dirname(path TEXT,
                                   separator CHAR DEFAULT '/')
    RETURNS TEXT AS
$$
DECLARE
    separare TEXT;
    dirname  TEXT;
    compname TEXT;
BEGIN
    IF POSITION(separator IN path) = 0 THEN
        RETURN '';
    END IF;
    separare := '^(.*' || separator || ').*$';
    dirname = REGEXP_REPLACE(path, separare, '\1');
    IF LENGTH(dirname) != 0 THEN
        compname := LPAD('', LENGTH(dirname), separator);
        IF compname != dirname THEN
            dirname = RTRIM(dirname, separator);
        END IF;
    END IF;
    RETURN dirname;
END;
$$
    LANGUAGE 'plpgsql' IMMUTABLE;
`

var basenameFunction = `
CREATE OR REPLACE FUNCTION basename(path TEXT,
                                    separator CHAR DEFAULT '/')
    RETURNS TEXT AS
$$
DECLARE
    separare TEXT;
BEGIN
    separare := '^.*' || separator;
    RETURN REGEXP_REPLACE(path, separare, '');
END;
$$
    LANGUAGE 'plpgsql' IMMUTABLE;
`

var dropFs = `
-- Dropping the functions
DROP FUNCTION IF EXISTS stat(TEXT);
DROP FUNCTION IF EXISTS ls(TEXT);
DROP FUNCTION IF EXISTS tree(TEXT);
DROP FUNCTION IF EXISTS touch(TEXT);
DROP FUNCTION IF EXISTS mkdir(TEXT);
DROP FUNCTION IF EXISTS mv(TEXT, TEXT);
DROP FUNCTION IF EXISTS rm(TEXT);
DROP FUNCTION IF EXISTS reset();
DROP FUNCTION IF EXISTS parseroot(TEXT);
DROP FUNCTION IF EXISTS validfname(TEXT);
DROP FUNCTION IF EXISTS sanitizefpath(TEXT, BOOL, TEXT);
DROP FUNCTION IF EXISTS parsesize(size BIGINT);
DROP FUNCTION IF EXISTS basename(path TEXT, separator CHAR);
DROP FUNCTION IF EXISTS dirname(path TEXT, separator CHAR);

-- Dropping the indexes
DROP INDEX IF EXISTS idx_fs_parent;
DROP INDEX IF EXISTS idx_fs_name;

-- Dropping the table
DROP TABLE IF EXISTS node;
DROP TABLE IF EXISTS fs CASCADE;
`

var refreshVFSFunction = `
CREATE OR REPLACE FUNCTION refresh_vfs()
    RETURNS void
    LANGUAGE plpgsql
AS
$$
DECLARE
    view_exists boolean;
BEGIN
    -- Check if the materialized view exists
    SELECT EXISTS (SELECT
                   FROM pg_catalog.pg_class c
                            JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
                   WHERE n.nspname = 'public' -- or your schema name
                     AND c.relname = 'vfs'
                     AND c.relkind = 'm' -- 'm' stands for materialized view
    )
    INTO view_exists;

    IF view_exists THEN
        -- Refresh the materialized view
        REFRESH MATERIALIZED VIEW vfs;
    ELSE
        -- Create the materialized view
        CREATE MATERIALIZED VIEW vfs AS
        WITH RECURSIVE vfs AS (SELECT *, fs.name::TEXT AS path
                               FROM fs
                               WHERE fs.parent IS NULL
                               UNION ALL
                               SELECT f.*, p.path || '/' || f.name AS path
                               FROM fs f
                                        JOIN vfs p ON f.parent = p.id)
        SELECT fs.id,
               parseroot(fs.path) as name,
               fs.dir,
               fs.size,
               fs.atime,
               fs.mtime,
               fs.parent
        FROM vfs fs;
        CREATE UNIQUE INDEX IF NOT EXISTS idx_vfs_name ON vfs (name);
        CREATE UNIQUE INDEX IF NOT EXISTS idx_vfs_name_parent ON vfs (name, parent);
    END IF;
END;
$$;
`

var statFunctionV2 = `
CREATE OR REPLACE FUNCTION stat(filepath TEXT)
    RETURNS TABLE
            (
                ID     UUID,
                NAME   TEXT,
                DIR    BOOL,
                SIZE   BIGINT,
                ATIME  TIMESTAMP,
                MTIME  TIMESTAMP,
                PARENT UUID
            )
AS
$$
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'stat');

    RETURN QUERY
        SELECT vfs.id, vfs.name, vfs.dir, vfs.size, vfs.atime, vfs.mtime, vfs.parent
        FROM vfs
        WHERE vfs.name = filepath;
END;
$$ LANGUAGE plpgsql;
`

var lsFunctionV2 = `
CREATE OR REPLACE FUNCTION ls(filepath TEXT)
    RETURNS TABLE
            (
                ID     UUID,
                NAME   TEXT,
                DIR    BOOL,
                SIZE   BIGINT,
                ATIME  TIMESTAMP,
                MTIME  TIMESTAMP,
                PARENT UUID
            )
AS
$$
DECLARE
    _id  UUID;
    _dir BOOL;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'ls');

    SELECT s.id, s.dir
    FROM stat(filepath) AS s
    INTO _id, _dir;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'ls % no such file or directory', filepath USING ERRCODE = 'P0002';
    END IF;

    IF _dir = FALSE THEN
        RAISE EXCEPTION 'ls % not a directory', filepath USING ERRCODE = 'P0004';
    END IF;

    RETURN QUERY
        SELECT vfs.id, vfs.name, vfs.dir, vfs.size, vfs.atime, vfs.mtime, vfs.parent
        FROM vfs
        WHERE vfs.name != filepath AND vfs.parent=_id;
END;
$$ LANGUAGE plpgsql;
`

var touchFunctionV2 = `
CREATE OR REPLACE FUNCTION touch(filepath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _id     UUID;
    _eid    UUID;
    _dir    BOOL;
    fname   TEXT;
    dirpath TEXT;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, TRUE, 'touch');
    dirpath := dirname(filepath);
    fname := basename(filepath);
    PERFORM validfname(fname::TEXT);

    SELECT s.id, s.dir
    FROM stat(dirpath) AS s
    INTO _id, _dir;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'touch % no such file or directory', dirpath USING ERRCODE = 'P0002';
    END IF;
    IF _dir = FALSE THEN
        RAISE EXCEPTION 'touch % not a directory', dirpath USING ERRCODE = 'P0004';
    END IF;

    SELECT ID FROM fs WHERE parent = _id AND name = fname INTO _eid;
    IF _eid IS NULL THEN
        INSERT INTO fs (name, dir, parent) VALUES (fname, FALSE, _id);
    END IF;

    EXECUTE refresh_vfs();
END;
$$ LANGUAGE plpgsql;
`

var mkdirFunctionV2 = `
CREATE OR REPLACE FUNCTION mkdir(filepath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _id        UUID;
    _parent_id UUID;
    _path      TEXT[] := STRING_TO_ARRAY(sanitizefpath(filepath, FALSE, 'mkdir'), '/');
    _name      TEXT;
    _dir       BOOL;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, FALSE, 'mkdir');

    -- Iterates over each part of the path
    FOR i IN 1..ARRAY_LENGTH(_path, 1)
        LOOP
            _name := _path[i];

            -- Tries to find the current part of the path in the parent directory
            SELECT id, dir
            INTO _id, _dir
            FROM fs
            WHERE name = _name
              AND (i = 1 OR parent = _parent_id);

            IF _dir = FALSE THEN
                RAISE EXCEPTION 'mkdir % no such file or directory', filepath USING ERRCODE = 'P0002';
            END IF;
            -- If the directory doesn't exist, create it
            IF _id IS NULL THEN
                INSERT INTO fs (name, dir, parent) VALUES (_name, TRUE, _parent_id) RETURNING id INTO _id;
            END IF;

            -- Sets the current directory as the parent for the next loop
            _parent_id := _id;
        END LOOP;

    EXECUTE refresh_vfs();
END;
$$ LANGUAGE plpgsql;
`

var mvFunctionV2 = `
CREATE OR REPLACE FUNCTION mv(oldpath TEXT, newpath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _old_id          UUID;
    _new_parent_id   UUID;
    _new_name        TEXT;
    _new_path        TEXT[] := STRING_TO_ARRAY(sanitizefpath(newpath, FALSE, 'mv'), '/');
    _new_parent_path TEXT;
BEGIN
    --- sanitize inputs
    oldpath = sanitizefpath(oldpath, FALSE, 'mv');
    newpath = sanitizefpath(newpath, FALSE, 'mv');

    -- If old path doesn't exist, raise an error
    SELECT s.id
    FROM stat(oldpath) AS s
    INTO _old_id;

    IF _old_id IS NULL THEN
        RAISE EXCEPTION 'mv % no such file or directory', oldpath USING ERRCODE = 'P0002';
    END IF;

    -- Split newpath into parent path and name
    _new_name := _new_path[ARRAY_LENGTH(_new_path, 1)];

    -- Construct the parent path manually
    _new_parent_path := '';
    FOR i IN 1..(ARRAY_LENGTH(_new_path, 1) - 1)
        LOOP
            _new_parent_path := _new_parent_path || '/' || _new_path[i];
        END LOOP;

    -- If new parent path doesn't exist, raise an error
    SELECT s.id
    FROM stat(_new_parent_path) AS s
    INTO _new_parent_id;

    IF _new_parent_id IS NULL THEN
        RAISE EXCEPTION 'mv % no such file or directory', newpath USING ERRCODE = 'P0002';
    END IF;

    -- Update the parent id and name of the old path
    UPDATE fs SET parent = _new_parent_id, name = _new_name WHERE id = _old_id;

    EXECUTE refresh_vfs();
END;
$$ LANGUAGE plpgsql;
`

var rmFunctionV2 = `
CREATE OR REPLACE FUNCTION rm(filepath TEXT)
    RETURNS VOID
AS
$$
DECLARE
    _id UUID;
BEGIN
    --- sanitize inputs
    filepath = sanitizefpath(filepath, FALSE, 'rm');

    SELECT s.id
    FROM stat(filepath) AS s
    INTO _id;

    IF _id IS NULL THEN
        RAISE EXCEPTION 'rm % no such file or directory',filepath USING ERRCODE = 'P0002';
    END IF;

    DELETE FROM fs WHERE id = _id;

    EXECUTE refresh_vfs();
END;
$$ LANGUAGE plpgsql;
`
