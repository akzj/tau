# A2 Audit Fragment Archive — doc 03 + 04 + 07

> Source: A2 final report turn ~95/100. Audited under coord-audit Q-First protocol.
> 
> See parent integration report `phase3-docs-audit.md`.

(See full A2 report content in coord-audit conversation. Key counts:
- doc 03: 6 ✅ / 2 ⚠️ / 2 ➕ / 0 ❌  
- doc 04: 7 ✅ / 4 ⚠️ / 3 ➕ / 1 ❌  
- doc 07: 8 ✅ / 3 ⚠️ / 2 ➕ / 0 ❌-但 7.6 行 215/219 整段标弃用)

Wire-Protocol 4/4 ✅ + G4 alternative compaction = 0 (维持砍).

A2 self-doubt #6 escalation: doc 07 行 132 `before_provider_payload` hook + onPayload/onResponse — 是否加 StreamRequest.OnPayload/OnResponse callback 字段。