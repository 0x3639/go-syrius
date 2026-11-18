# Embedded node upgrade and recovery

go-syrius now embeds the official [go-zenon v0.0.10 release](https://github.com/zenon-network/go-zenon/releases/tag/v0.0.10-alphanet). The upgrade reuses the existing `embedded` directory. It does not delete or migrate the chain database automatically.

An older node that followed a divergent chain may need a full resync after upgrading. This is a conditional recovery procedure, not a requirement for every upgrade. The upstream [v0.0.9 operator guidance](https://forum.hypercore.one/t/pillars-upgrade-go-zenon-to-v0-0-9-now/944) describes why nodes that miss a protocol activation can need a resync. A displayed height or “Synced” label alone does not establish agreement with the canonical chain; compare momentum hashes at the same confirmed height with trusted, up-to-date nodes before diagnosing a fork.

## Recover through Settings

1. In **Settings → Node**, select **Remote**, enter a trusted endpoint for the intended network, and choose **Apply node**. Then quit go-syrius completely and verify that its process has exited. A successful Remote connection does not prove that embedded shutdown completed.
2. With the process stopped, optionally copy only its `embedded` directory to a separate backup location for diagnosis. Reopen go-syrius in Remote mode before continuing. Never copy a database while the embedded node is writing to it.
3. Choose **Delete embedded data**. This removes the app's downloaded node data and requires a full download again. It does not remove the wallet keystores or settings. Do not delete the parent application data directory.
4. Select **Embedded**, choose **Apply node**, and confirm **Start embedded**. Allow the node to resync; this can take hours and several GB of disk space. Check peer connectivity, progress, and agreement with trusted nodes before relying on the recovered node.

The app refuses deletion while the node is running, stopping, or has failed to stop cleanly. Keep using a trusted Remote endpoint if embedded recovery cannot complete.

## Validation status for this upgrade

Automated tests cover clean-stop requirements, rejection after a timed-out stop, and deletion scoped to the embedded directory. Fresh embedded startup/RPC/shutdown was checked with v0.0.10. Recovery from a copy of a real pre-upgrade divergent database has **not** been validated; no such fixture is available in this repository. Before a release claims verified migration from that state, run the steps above on an isolated copy, confirm canonical momentum hashes after resync, and record the source version and results. Never use the only copy of a user's database for this acceptance check.
