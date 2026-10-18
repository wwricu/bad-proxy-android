package icu.wwr.badproxy

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.IBinder
import android.util.Log
import vpn.Vpn

class BpVpnService : VpnService() {

    private var sessionLabel: String? = null

    override fun onBind(intent: Intent?): IBinder? = super.onBind(intent)

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopTunnel()
            ACTION_CONNECT -> {
                val cfg = intent.getStringExtra(EXTRA_CONFIG)
                if (cfg != null) startTunnel(cfg, intent.getStringExtra(EXTRA_LABEL)) else stopTunnel()
            }
            // On the first connection the framework starts this service itself
            // (action android.net.VpnService, from the consent dialog) and that
            // start can land after we have already established the tunnel.
            // Stopping there tears down the VPN the user just approved, so any
            // start we do not recognise keeps the running tunnel alive.
            else -> if (Vpn.running()) promote() else stopSelf()
        }
        return START_NOT_STICKY
    }

    private fun startTunnel(cfgJson: String, label: String?) {
        if (Vpn.running()) return
        sessionLabel = label
        // The tunnel is a long-lived user-visible feature: run it as a
        // foreground service (also what keeps the process alive once the
        // activity goes away on newer Android).
        promote()
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
            stopTunnel()
            return
        }
        if (pfd == null) {
            Log.e(TAG, "tun establish refused")
            stopTunnel()
            return
        }
        try {
            // detachFd transfers ownership of the fd to the Go side; Stop()
            // closes it.
            Vpn.start(cfgJson, pfd.detachFd().toLong())
        } catch (e: Exception) {
            Log.e(TAG, "engine start failed", e)
            stopTunnel()
        }
    }

    private fun promote() {
        startForeground(NOTIF_ID, buildNotification(sessionLabel))
    }

    private fun stopTunnel() {
        Vpn.stop()
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        super.onDestroy()
        Vpn.stop()
    }

    private fun buildNotification(label: String?): Notification {
        ensureChannel()
        val contentIntent = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or pendingIntentImmutable(),
        )
        val stopIntent = PendingIntent.getService(
            this, 1, Intent(this, BpVpnService::class.java).setAction(ACTION_STOP),
            PendingIntent.FLAG_UPDATE_CURRENT or pendingIntentImmutable(),
        )
        val text = if (label.isNullOrEmpty()) "VPN connected" else "Connected · $label"
        val nb = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL)
        } else {
            @Suppress("DEPRECATION")
            Notification.Builder(this)
        }
        return nb.setContentTitle(getString(R.string.notif_title))
            .setContentText(text)
            .setSmallIcon(R.drawable.ic_shield)
            .setOngoing(true)
            .setContentIntent(contentIntent)
            .addAction(R.drawable.ic_shield, getString(R.string.disconnect), stopIntent)
            .build()
    }

    private fun ensureChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (nm.getNotificationChannel(CHANNEL) == null) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL, getString(R.string.notif_channel), NotificationManager.IMPORTANCE_LOW)
            )
        }
    }

    companion object {
        const val ACTION_CONNECT = "icu.wwr.badproxy.action.CONNECT"
        const val ACTION_STOP = "icu.wwr.badproxy.action.STOP"
        const val EXTRA_CONFIG = "icu.wwr.badproxy.extra.CONFIG"
        const val EXTRA_LABEL = "icu.wwr.badproxy.extra.LABEL"

        private const val TAG = "BpVpnService"
        private const val TUN_ADDRESS = "10.255.0.2"
        private const val TUN_PREFIX = 24
        private const val TUN_DNS = "8.8.8.8"
        private const val TUN_MTU = 1500
        private const val NOTIF_ID = 1
        private const val CHANNEL = "vpn"

        fun isRunning(): Boolean = Vpn.running()

        fun lastError(): String = Vpn.lastError()

        // The service promotes itself to foreground, so every entry point
        // must use startForegroundService on O+ to get the grace window.
        fun startServiceForeground(ctx: Context, intent: Intent) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                ctx.startForegroundService(intent)
            } else {
                ctx.startService(intent)
            }
        }

        private fun pendingIntentImmutable(): Int =
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) PendingIntent.FLAG_IMMUTABLE else 0
    }
}
