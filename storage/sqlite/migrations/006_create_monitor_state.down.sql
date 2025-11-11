-- ============================================================================
-- Migration: 006_create_monitor_state.down.sql
-- Purpose: Reverse the 006_create_monitor_state.up.sql migration
-- ============================================================================
--
-- This migration removes the monitor_state table and its associated index.
-- All monitor state data will be permanently deleted.
--
-- WARNING: This operation is destructive and will delete all monitor state,
--          including configuration, statistics, and last seen post IDs.
--
-- ============================================================================

-- Drop index first (must be dropped before the table)
DROP INDEX IF EXISTS idx_monitor_state_status;

-- Drop the monitor_state table
DROP TABLE IF EXISTS monitor_state;
