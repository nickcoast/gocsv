-- Table: public.core_raw_tables
-- UPS
CREATE TABLE IF NOT EXISTS public.core_raw_tables (
    id SERIAL,
    name character varying(63) COLLATE pg_catalog."default",
    format_id bigint,
    row_count integer,
    row_count_dedup integer,
    key_field_id BIGINT NULL DEFAULT NULL,
    key_count_old BIGINT NULL DEFAULT NULL,
    key_count_new BIGINT NULL DEFAULT NULL,
    key_count_unique BIGINT NULL DEFAULT NULL,
    key_count_dup BIGINT NULL DEFAULT NULL,
    deleted boolean NOT NULL DEFAULT false,
    source_filename character varying(255) COLLATE pg_catalog."default" NOT NULL,
    file_size bigint NOT NULL,
    datetime_uploaded timestamp with time zone NOT NULL,
    saved_filename character varying(255) COLLATE pg_catalog."default",
    file_hash bytea,
    file_hash_no_bom bytea,
    file_hash_trimmed_no_bom bytea,
    CONSTRAINT core_raw_tables_pkey PRIMARY KEY (id)
) TABLESPACE pg_default;
ALTER TABLE IF EXISTS public.core_raw_tables OWNER to postgres;
GRANT DELETE,
    INSERT,
    SELECT,
    UPDATE ON TABLE public.core_raw_tables TO ogrego;
GRANT ALL ON TABLE public.core_raw_tables TO postgres;
COMMENT ON COLUMN public.core_raw_tables.name IS 'DB table name containing data from the uploaded csv';
COMMENT ON COLUMN public.core_raw_tables.saved_filename IS 'name as saved on disk';
COMMENT ON COLUMN public.core_raw_tables.file_hash IS 'hash of file contents';
COMMENT ON COLUMN public.core_raw_tables.file_hash_no_bom IS 'hash of file contents excluding BOM';
COMMENT ON COLUMN public.core_raw_tables.file_hash_trimmed_no_bom IS 'hash of file contents after removing empty rows and columns, excluding BOM';
COMMENT ON COLUMN public.core_raw_tables.row_count IS 'Count of total rows in table including any header row';
COMMENT ON COLUMN public.core_raw_tables.row_count_dedup IS 'Count of rows in table after deduplication';
COMMENT ON COLUMN public.core_raw_tables.deleted IS 'Is file deleted';
COMMENT ON COLUMN public.core_raw_tables.format_id IS 'FK to formats table';
--- SEQ
-- SEQUENCE: public.core_raw_tables_id_seq
-- DROP SEQUENCE IF EXISTS public.core_raw_tables_id_seq;
CREATE SEQUENCE IF NOT EXISTS public.core_raw_tables_id_seq INCREMENT 1 START 1 MINVALUE 1 MAXVALUE 2147483647 CACHE 1 OWNED BY core_raw_tables.id;
ALTER SEQUENCE public.core_raw_tables_id_seq OWNER TO postgres;
GRANT SELECT,
    USAGE ON SEQUENCE public.core_raw_tables_id_seq TO ogrego;
GRANT ALL ON SEQUENCE public.core_raw_tables_id_seq TO postgres;