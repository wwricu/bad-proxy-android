package icu.wwr.badproxy

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.content.pm.PackageManager
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.BaseAdapter
import android.widget.Button
import android.widget.CheckBox
import android.widget.ListView
import android.widget.RadioGroup
import android.widget.TextView
import android.widget.Toast
import vpn.Vpn

class MainActivity : Activity() {

    private lateinit var prefs: Prefs
    private lateinit var nodesList: ListView
    private lateinit var routing: RadioGroup
    private lateinit var connectBtn: Button
    private lateinit var tunnelTest: Button
    private lateinit var autoConnect: CheckBox
    private lateinit var status: TextView
    private lateinit var nodesAdapter: NodesAdapter

    // Per-node latency results from the last manual test, index -> chip text
    // ("12ms" / "Testing…" / "Failed"), same lifecycle as the desktop chip.
    private val latency = mutableMapOf<Int, String>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        prefs = Prefs(this)
        nodesList = findViewById(R.id.nodes)
        routing = findViewById(R.id.routing)
        connectBtn = findViewById(R.id.connect)
        tunnelTest = findViewById(R.id.tunnel_test)
        autoConnect = findViewById(R.id.auto_connect)
        status = findViewById(R.id.status)

        routing.check(
            when (prefs.state.routingMode) {
                "cn-whitelist" -> R.id.route_cn
                "gfw-blacklist" -> R.id.route_gfw
                else -> R.id.route_global
            }
        )
        routing.setOnCheckedChangeListener { _, id ->
            prefs.state.routingMode = when (id) {
                R.id.route_cn -> "cn-whitelist"
                R.id.route_gfw -> "gfw-blacklist"
                else -> "global"
            }
            prefs.save()
        }

        nodesAdapter = NodesAdapter()
        nodesList.adapter = nodesAdapter
        nodesList.setOnItemClickListener { _, _, pos, _ ->
            prefs.state.active = pos
            prefs.save()
            refresh()
        }
        nodesList.setOnItemLongClickListener { _, _, pos, _ ->
            nodeMenu(pos)
            true
        }

        findViewById<Button>(R.id.add_node).setOnClickListener {
            NodeDialog(this) { node ->
                prefs.state.nodes.add(node)
                prefs.state.active = prefs.state.nodes.size - 1
                prefs.save()
                latency.clear()
                refresh()
            }.show()
        }
        tunnelTest.setOnClickListener { testTunnel() }
        autoConnect.isChecked = prefs.state.autoConnect
        autoConnect.setOnCheckedChangeListener { _, checked ->
            prefs.state.autoConnect = checked
            prefs.save()
        }

        connectBtn.setOnClickListener {
            if (BpVpnService.isRunning()) disconnect() else connect()
        }
        refresh()
    }

    override fun onResume() {
        super.onResume()
        visible = true
        refresh()
        // The service starts asynchronously and the engine can die on its own,
        // so the status line is polled rather than set once at click time.
        handler.postDelayed(ticker, TICK_MS)
    }

    override fun onPause() {
        super.onPause()
        visible = false
        handler.removeCallbacks(ticker)
    }

    private var visible = false
    private val handler = Handler(Looper.getMainLooper())
    private val ticker = object : Runnable {
        override fun run() {
            // A callback can still be in flight across a pause (the VPN
            // consent dialog does this); touching detached views spams
            // "View not attached" through the Choreographer.
            if (!visible) return
            refreshStatus()
            handler.postDelayed(this, TICK_MS)
        }
    }

    private fun nodeMenu(pos: Int) {
        val labels = arrayOf(getText(R.string.test_node), getText(R.string.edit_node), getText(R.string.delete_node))
        AlertDialog.Builder(this)
            .setTitle(prefs.state.nodes[pos].name)
            .setItems(labels) { _, which ->
                when (which) {
                    0 -> testNode(pos)
                    1 -> editNode(pos)
                    2 -> confirmDelete(pos)
                }
            }
            .show()
    }

    private fun editNode(pos: Int) {
        NodeDialog(this) { node ->
            prefs.state.nodes[pos] = node
            prefs.save()
            latency.remove(pos)
            refresh()
        }.show(prefs.state.nodes[pos])
    }

    private fun confirmDelete(pos: Int) {
        AlertDialog.Builder(this)
            .setMessage(R.string.confirm_delete)
            .setPositiveButton(android.R.string.ok) { _, _ ->
                prefs.state.nodes.removeAt(pos)
                prefs.state.active = if (prefs.state.nodes.isEmpty()) -1 else 0.coerceAtMost(pos)
                prefs.save()
                latency.clear()
                refresh()
            }
            .setNegativeButton(android.R.string.cancel, null)
            .show()
    }

    // testNode mirrors the desktop TestNode: throwaway engine, node as the
    // only outbound, real 204 request; the binding call blocks up to its
    // timeout, so it runs off the main thread and lands back on it.
    private fun testNode(pos: Int) {
        if (latency[pos] == getString(R.string.testing)) return
        latency[pos] = getString(R.string.testing)
        nodesAdapter.notifyDataSetChanged()
        val cfg = prefs.vpnConfigJson(filesDir, pos)
        Thread {
            val result = try {
                Vpn.testNode(cfg)
            } catch (e: Exception) {
                android.util.Log.w("BpTest", "testNode failed", e)
                handler.post { toast("Test failed: ${e.message}") }
                "Failed"
            }
            handler.post {
                latency[pos] = result
                nodesAdapter.notifyDataSetChanged()
            }
        }.start()
    }

    private fun testTunnel() {
        tunnelTest.isEnabled = false
        Thread {
            val result = try {
                "tunnel " + Vpn.testConnection()
            } catch (e: Exception) {
                android.util.Log.w("BpTest", "testConnection failed", e)
                "tunnel failed: ${e.message}"
            }
            handler.post {
                tunnelTest.isEnabled = true
                toast(result)
            }
        }.start()
    }

    private fun toast(text: String) {
        Toast.makeText(this, text, Toast.LENGTH_LONG).show()
    }

    private fun refresh() {
        nodesAdapter.notifyDataSetChanged()
        refreshStatus()
    }

    private fun refreshStatus() {
        val running = BpVpnService.isRunning()
        connectBtn.setText(if (running) R.string.disconnect else R.string.connect)
        tunnelTest.visibility = if (running) View.VISIBLE else View.GONE
        val active = prefs.state.active.takeIf { it in prefs.state.nodes.indices }
        val route = when (prefs.state.routingMode) {
            "cn-whitelist" -> "CN whitelist"
            "gfw-blacklist" -> "GFW blacklist"
            else -> "Global"
        }
        status.text = when {
            running -> buildString {
                append("Connected")
                active?.let { append(" · ").append(prefs.state.nodes[it].name) }
                append(" · ").append(route)
            }
            BpVpnService.lastError().isNotEmpty() -> "Failed: " + BpVpnService.lastError()
            else -> "Idle · ${route}"
        }
    }

    private inner class NodesAdapter : BaseAdapter() {
        override fun getCount() = prefs.state.nodes.size
        override fun getItem(position: Int) = prefs.state.nodes[position]
        override fun getItemId(position: Int) = position.toLong()

        override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {
            val v = convertView ?: LayoutInflater.from(this@MainActivity)
                .inflate(R.layout.node_row, parent, false)
            val n = prefs.state.nodes[position]
            v.findViewById<TextView>(R.id.node_name).text = n.name
            v.findViewById<TextView>(R.id.node_summary).text =
                "${n.protocol}://${n.server}:${n.port} · ${n.transport}"
            v.findViewById<TextView>(R.id.node_latency).text = latency[position] ?: ""
            // The desktop list highlights the active row; do the same instead
            // of relying on ListView choice mode with a custom adapter.
            v.setBackgroundColor(
                if (position == prefs.state.active) 0xFFE8F0FE.toInt() else 0x00000000
            )
            return v
        }
    }

    private fun connect() {
        if (prefs.state.active !in prefs.state.nodes.indices) {
            toast("Add and select a node first")
            return
        }
        // Both prompts are system dialogs started with startActivityForResult,
        // so they must run one after the other: asking for the notification
        // permission while the VPN consent dialog is in flight swallows the
        // consent result and the tunnel never starts.
        if (needsNotificationPermission()) {
            requestPermissions(arrayOf("android.permission.POST_NOTIFICATIONS"), REQ_NOTIF)
            return
        }
        requestVpnConsent()
    }

    private fun needsNotificationPermission(): Boolean =
        Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission("android.permission.POST_NOTIFICATIONS") !=
            PackageManager.PERMISSION_GRANTED

    private fun requestVpnConsent() {
        val prep = VpnService.prepare(this)
        if (prep != null) {
            startActivityForResult(prep, REQ_VPN)
            return
        }
        startVpn()
    }

    private fun startVpn() {
        prefs.ensureRules(this)
        val svc = Intent(this, BpVpnService::class.java).apply {
            action = BpVpnService.ACTION_CONNECT
            putExtra(BpVpnService.EXTRA_CONFIG, prefs.vpnConfigJson(filesDir))
            prefs.state.active.takeIf { it in prefs.state.nodes.indices }?.let {
                putExtra(BpVpnService.EXTRA_LABEL, prefs.state.nodes[it].name)
            }
        }
        BpVpnService.startServiceForeground(this, svc)
        refresh()
    }

    private fun disconnect() {
        BpVpnService.startServiceForeground(
            this,
            Intent(this, BpVpnService::class.java).apply { action = BpVpnService.ACTION_STOP }
        )
        refresh()
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQ_VPN) {
            if (resultCode == RESULT_OK) startVpn()
            else toast("VPN permission denied")
        }
    }

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        // Whether notifications were allowed or not makes no difference to the
        // tunnel; only its notification would be hidden. Carry on either way.
        if (requestCode == REQ_NOTIF) requestVpnConsent()
    }

    companion object {
        private const val REQ_VPN = 1
        private const val REQ_NOTIF = 2
        private const val TICK_MS = 1000L
    }
}
