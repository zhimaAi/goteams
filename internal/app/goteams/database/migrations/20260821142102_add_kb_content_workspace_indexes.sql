-- +goose Up
-- +goose StatementBegin

DROP INDEX IF EXISTS uk_knowledge_contents_document;
CREATE UNIQUE INDEX uk_knowledge_contents_document
    ON knowledge_document_contents(admin_id, workspace_id, document_id);

DROP INDEX IF EXISTS uk_knowledge_link_contents;
CREATE UNIQUE INDEX uk_knowledge_link_contents
    ON knowledge_link_contents(admin_id, workspace_id, link_id);

DROP INDEX IF EXISTS uk_knowledge_version_no;
CREATE UNIQUE INDEX uk_knowledge_version_no
    ON knowledge_document_versions(admin_id, workspace_id, document_id, version_no);

DROP INDEX IF EXISTS uk_knowledge_version_contents;
CREATE UNIQUE INDEX uk_knowledge_version_contents
    ON knowledge_document_version_contents(admin_id, workspace_id, version_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS uk_knowledge_link_contents;
CREATE UNIQUE INDEX uk_knowledge_link_contents
    ON knowledge_link_contents(admin_id, link_id);

DROP INDEX IF EXISTS uk_knowledge_version_contents;
CREATE UNIQUE INDEX uk_knowledge_version_contents
    ON knowledge_document_version_contents(admin_id, version_id);

DROP INDEX IF EXISTS uk_knowledge_version_no;
CREATE UNIQUE INDEX uk_knowledge_version_no
    ON knowledge_document_versions(admin_id, document_id, version_no);

DROP INDEX IF EXISTS uk_knowledge_contents_document;
CREATE UNIQUE INDEX uk_knowledge_contents_document
    ON knowledge_document_contents(admin_id, document_id);

-- +goose StatementEnd
