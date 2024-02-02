package postgres

import "github.com/forscht/ddrv/pkg/migrate"

var migrations = []migrate.Migration{
	{
		ID: 1,
		Up: migrate.Queries([]string{
			fsTable,
			nodeTable,
			fsParentIdx,
			fsNameIdx,
			rootInsert,
			statFunction,
			lsFunction,
			treeFunction,
			touchFunction,
			mkdirFunction,
			mvFunction,
			rmFunction,
			resetFunction,
			parserootFunction,
			validnameFunction,
			sanitizeFPath,
			parseSizeFunction,
			basenameFunction,
			dirnameFunction,
		}),
		Down: migrate.Queries([]string{dropFs}),
	},
	{
		ID: 2,
		Up: migrate.Queries([]string{
			`
				CREATE TABLE temp_node
				(
				    id    BIGINT PRIMARY KEY NOT NULL,
				    file  UUID               NOT NULL REFERENCES fs (id) ON DELETE CASCADE,
				    url   VARCHAR(255)       NOT NULL,
				    size  INTEGER            NOT NULL,
				    iv    VARCHAR(255)       NOT NULL DEFAULT '',
				    mtime TIMESTAMP          NOT NULL DEFAULT NOW()
				);
				
				INSERT INTO temp_node (id, file, url, size, iv, mtime)
				SELECT CAST(
				               (REGEXP_MATCHES(url, '/([0-9]+)/[A-Za-z0-9_-]+$', 'g'))[1]
				           AS BIGINT) AS id,
				       file,
				       url,
				       size,
				       iv,
				       mtime
				FROM node;
				
				DROP TABLE node;
				
				ALTER TABLE temp_node RENAME TO node;
				
				alter table public.node rename constraint temp_node_pkey to node_pkey;
				
				alter table public.node rename constraint temp_node_file_fkey to node_file_fkey;
			`,
		}),
		Down: migrate.Queries([]string{}),
	},
	{
		ID:   3,
		Up:   migrate.Queries([]string{`CREATE INDEX idx_node_file ON node (file);`}),
		Down: migrate.Queries([]string{`DROP INDEX idx_node_file;`}),
	},
	{
		ID:   4,
		Up:   migrate.Queries([]string{`CREATE INDEX idx_node_size ON node (size);`}),
		Down: migrate.Queries([]string{`DROP INDEX idx_node_size;`}),
	},
	{
		ID:   5,
		Up:   migrate.Queries([]string{`ALTER TABLE node ADD COLUMN mid BIGINT, ADD COLUMN ex INT, ADD COLUMN "is" INT, ADD COLUMN hm VARCHAR(255);`}),
		Down: migrate.Queries([]string{`ALTER TABLE node DROP COLUMN mid, DROP COLUMN ex, DROP COLUMN "is", DROP COLUMN hm;`}),
	},
	{
		ID:   6,
		Up:   migrate.Queries([]string{`CREATE UNIQUE INDEX IF NOT EXISTS idx_node_mid_unique ON node(mid);`}),
		Down: migrate.Queries([]string{`DROP INDEX IF EXISTS idx_node_mid_unique;`}),
	},
	{
		ID: 7,
		Up: migrate.Queries([]string{`
			CREATE OR REPLACE FUNCTION validfname(filename TEXT)
			    RETURNS VOID
			AS
			$$
			BEGIN
			    -- first, check if the filename is not NULL or an empty string.
			    -- next, check if it does not start with a space.
			    -- then, check if the filename doesn't contain any invalid characters (like /, <, >, :, ", |, or *).
			    -- finally, if all conditions are met, return true; otherwise, return false.
			    IF filename IS NOT NULL AND filename != '' AND filename !~ '^ ' AND filename !~ '[/<>"\|\*]' THEN
			    ELSE
			        RAISE EXCEPTION 'invalid filename %', filename USING ERRCODE = 'P0002';
			    END IF;
			END;
			$$ LANGUAGE plpgsql;
			`}),
		Down: migrate.Queries([]string{`
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
			`}),
	},
	{
		ID: 8,
		Up: migrate.Queries([]string{
			`ALTER TABLE fs ADD COLUMN size bigint DEFAULT 0 NOT NULL;`,
			`UPDATE fs SET size = COALESCE((SELECT SUM(node.size)FROM node WHERE node.file = fs.id), 0);`,
			refreshVFSFunction,
			statFunctionV2,
			lsFunctionV2,
			touchFunctionV2,
			mkdirFunctionV2,
			mvFunctionV2,
			rmFunctionV2,
			`DROP FUNCTION IF EXISTS tree(TEXT);`,
			`DROP FUNCTION IF EXISTS parsesize(size BIGINT);`,
			`SELECT * FROM refresh_vfs();`,
		}),
		Down: migrate.Queries([]string{
			`ALTER TABLE fs DROP COLUMN size;`,
			statFunction,
			lsFunction,
			touchFunction,
			mkdirFunction,
			mvFunction,
			rmFunction,
			treeFunction,
			parseSizeFunction,
			`DROP FUNCTION IF EXIST refresh_vfs();`,
		}),
	},
}
