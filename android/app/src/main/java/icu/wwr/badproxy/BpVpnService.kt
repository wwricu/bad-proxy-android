package icu.wwr.badproxy

import android.content.Intent
import android.net.VpnService
import android.os.IBinder
import android.util.Log
import vpn.Vpn

class BpVpnService : VpnService() {

    override fun onBind(intent: Intent?): IBinder? = super.onBind(intent)

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopTunnel()
            ACTION_CONNECT -> {
                val cfg = intent.getStringExtra(EXTRA_CONFIG)
                if (cfg != null) startTunnel(cfg) else stopSelf()
            }
            else -> stopSelf()
        }
        return START_NOT_STICKY
    }

    private fun startTunnel(cfgJson: String) {
        if (Vpn.running()) return
        val pfd = try {
            Builder()
                .setSession("Bad Proxy")
                .addAddress(TUN_ADDRESS, TUN_PREFIX)
                .addRoute("0.0.0.0", 0)
                .addDnsServer(TUN_DNS)
                .setMtu(TUN_MTU)
                // The owner uid is not kept off the tunnel it created: an
                // unmarked socket from this process matches the
                // `fwmark 0x0/0x20000` rule, so without this entry the
                // engine's own resolver query is answered by the fake DNS on
                // the tun and every direct dial lands on a 198.18.x.x
                // placeholder. (The allow list does the opposite of what its
                // name suggests: it binds the listed uid *to* the VPN.)
                // Verified by the per-uid rules losing the 10216-10216 range.
                .addDisallowedApplication(packageName)
                .setBlocking(true)
                .establish()
        } catch (e: Exception) {
            Log.e(TAG, "tun establish failed", e)
            stopSelf()
            return
        }
        if (pfd == null) {
            Log.e(TAG, "tun establish refused")
            stopSelf()
            return
        }
        try {
            // detachFd transfers ownership of the fd to the Go side; Stop()
            // closes it.
            Vpn.start(cfgJson, pfd.detachFd().toLong())
        } catch (e: Exception) {
            Log.e(TAG, "engine start failed", e)
            stopSelf()
        }
    }

    private fun stopTunnel() {
        Vpn.stop()
        stopSelf()
    }

    override fun onDestroy() {
        super.onDestroy()
        Vpn.stop()
    }

    companion object {
        const val ACTION_CONNECT = "icu.wwr.badproxy.action.CONNECT"
        const val ACTION_STOP = "icu.wwr.badproxy.action.STOP"
        const val EXTRA_CONFIG = "icu.wwr.badproxy.extra.CONFIG"

        private const val TAG = "BpVpnService"
        private const val TUN_ADDRESS = "10.255.0.2"
        private const val TUN_PREFIX = 24
        private const val TUN_DNS = "8.8.8.8"
        private const val TUN_MTU = 1500

        fun isRunning(): Boolean = Vpn.running()

        fun lastError(): String = Vpn.lastError()
    }
}
