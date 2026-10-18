package icu.wwr.badproxy

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.VpnService

// BootReceiver is the device-side equivalent of the desktop app's
// "start at login" setting, but wired to connect rather than merely launch:
// a VPN that opens its window and waits is useless at boot.
class BootReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return
        val prefs = Prefs(context)
        if (!prefs.state.autoConnect) return
        if (prefs.state.active !in prefs.state.nodes.indices) return
        // Non-null here means the user has never granted (or has revoked)
        // the VPN consent, which cannot be re-granted without UI; skip.
        if (VpnService.prepare(context) != null) return
        prefs.ensureRules(context)
        val svc = Intent(context, BpVpnService::class.java).apply {
            action = BpVpnService.ACTION_CONNECT
            putExtra(BpVpnService.EXTRA_CONFIG, prefs.vpnConfigJson(context.filesDir))
            prefs.state.active.takeIf { it in prefs.state.nodes.indices }?.let {
                putExtra(BpVpnService.EXTRA_LABEL, prefs.state.nodes[it].name)
            }
        }
        BpVpnService.startServiceForeground(context, svc)
    }
}
