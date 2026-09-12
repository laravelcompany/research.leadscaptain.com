-- Attribute leads to the objective that discovered them so per-objective
-- progress and completion are measured against the right set of leads.
ALTER TABLE leads ADD COLUMN objective_id INTEGER REFERENCES objectives(id);
CREATE INDEX IF NOT EXISTS idx_leads_objective ON leads(objective_id);
