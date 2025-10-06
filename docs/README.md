# MCP-Compose Documentation Archive

This directory contains historical documentation from the development process.

## 📋 Current Status Documents

**Start Here:**
- **[TODO.md](../TODO.md)** - ⚠️ **CRITICAL ISSUES & FIXES REQUIRED** - Read this FIRST
- **[README.md](../README.md)** - Main project documentation
- **[CLAUDE.md](../CLAUDE.md)** - Instructions for Claude Code agents

## 📁 Document Categories

### Critical Issue Reports
- **CLAUDE_DESTROYED_PROJECT_NOTES.md** - Critical incident report from git checkout disaster
  - Documents what was lost and what needs to be rebuilt
  - Recovery strategy and testing checklist
  - **NOTE:** Many issues described here have since been fixed (see TODO.md for current status)

### Implementation Summaries
- **IMPLEMENTATION_SUMMARY.md** - Complete task-chat integration implementation
- **WEBSOCKET_IMPLEMENTATION.md** - Bidirectional real-time WebSocket chat
- **CHAT_INTEGRATION.md** - MCP tool awareness system
- **TASK_SCHEDULER_PROGRESS.md** - Task scheduler progress tracking

### Fix Logs & Verifications
- **FIXLOG.md** - Task scheduler chat integration fixes
- **NEW_CHAT_FIX_VERIFICATION.md** - New chat connection fix verification
- **MCP_SELECTION_FIX.md** - MCP selection UI fix
- **PROVIDER_INHERITANCE_TEST.md** - Provider/model inheritance testing
- **REGISTRY_SCHEMA_FIX.md** - Registry database schema fixes
- **TEST_CHAT.md** - Chat testing documentation

### Feature Documentation
- **REGISTRY.md** - MCP Server Registry feature
- **REGISTRY_IMPLEMENTATION.md** - Registry implementation details
- **REGISTRY_INTEGRATION.md** - Registry integration guide
- **TASK_CHAT.md** - Task-chat integration details
- **GETTING-STARTED.md** - Getting started guide

## 🎯 For New Developers/Agents

1. **Read [TODO.md](../TODO.md) first** - It has the most current and accurate status
2. **Then read CLAUDE_DESTROYED_PROJECT_NOTES.md** - Understand what was broken
3. **Then read implementation docs** - See what was built and how

## ⚠️ Important Notes

### Document Accuracy
- **Most Accurate:** TODO.md (just created, reflects current reality)
- **Historical Context:** CLAUDE_DESTROYED_PROJECT_NOTES.md (describes problems, many now fixed)
- **Implementation Details:** Implementation summary docs (describe how things work)
- **Fix Logs:** May describe issues that have since been fixed or new issues found

### Known Outdated Information
Some documents describe issues as "broken" or "fixed" but the current status may have changed:
- WebSocket connections described as broken → Partially fixed (see TODO.md Issue #2)
- Message persistence described as working → **Actually broken** (see TODO.md Issue #1)
- Task output described as complete → Partially working (see TODO.md Issue #3)

**Always check TODO.md for the actual current status of any feature.**

## 📞 Quick Reference

### If you're debugging chat issues:
1. Read TODO.md Issues #1, #2, #3
2. Read WEBSOCKET_IMPLEMENTATION.md
3. Read CLAUDE_DESTROYED_PROJECT_NOTES.md sections on WebSocket and Chat

### If you're working on task scheduler:
1. Read TODO.md Issue #3
2. Read FIXLOG.md
3. Read TASK_SCHEDULER_PROGRESS.md

### If you're working on the registry:
1. Read REGISTRY.md
2. Read REGISTRY_IMPLEMENTATION.md
3. Check TODO.md Issue #7

---

**Last Updated:** 2025-10-03
**Maintained By:** Development team (auto-generated from root documentation)
