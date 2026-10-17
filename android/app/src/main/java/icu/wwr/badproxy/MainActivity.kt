package icu.wwr.badproxy

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.widget.ArrayAdapter
import android.widget.Button
import android.widget.ListView
import android.widget.RadioButton
import android.widget.RadioGroup
import android.widget.TextView
import android.widget.Toast

class MainActivity : Activity() {

    private lateinit var prefs: Prefs
    private lateinit var nodesList: ListView
    private lateinit var routing: RadioGroup
    private lateinit var connectBtn: Button
    private lateinit var status: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        prefs = Prefs(this)
        nodesList = findViewById(R.id.nodes)
        routing = findViewById(R.id.routing)
        connectBtn = findViewById(R.id.connect)
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

        nodesList.setOnItemClickListener { _, _, pos, _ ->
            prefs.state.active = pos
            prefs.save()
            refresh()
        }
        findViewById<Button>(R.id.add_node).setOnClickListener {
            NodeDialog(this) { node ->
                prefs.state.nodes.add(node)
                prefs.state.active = prefs.state.nodes.size - 1
                prefs.save()
                refresh()
            }.show()
        }
        findViewById<Button>(R.id.edit_node).setOnClickListener {
            val i = prefs.state.active
            if (i in prefs.state.nodes.indices) {
                NodeDialog(this) { node ->
                    prefs.state.nodes[i] = node
                    prefs.save()
                    refresh()
                }.show(prefs.state.nodes[i])
            }
        }
        findViewById<Button>(R.id.delete_node).setOnClickListener {
            val i = prefs.state.active
            if (i in prefs.state.nodes.indices) {
                prefs.state.nodes.removeAt(i)
                prefs.state.active = if (prefs.state.nodes.isEmpty()) -1 else 0
                prefs.save()
                refresh()
            }
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
    private val handler = android.os.Handler(android.os.Looper.getMainLooper())
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

    private fun refresh() {
        val labels = prefs.state.nodes.mapIndexed { i, n ->
            val star = if (i == prefs.state.active) "* " else "  "
            "$star${n.name}  (${n.protocol}@${n.server}:${n.port})"
        }
        nodesList.adapter = ArrayAdapter(this, android.R.layout.simple_list_item_single_choice, labels)
        nodesList.choiceMode = ListView.CHOICE_MODE_SINGLE
        if (prefs.state.active in prefs.state.nodes.indices) {
            nodesList.setItemChecked(prefs.state.active, true)
        }
        refreshStatus()
    }

    private fun refreshStatus() {
        val running = BpVpnService.isRunning()
        connectBtn.setText(if (running) R.string.disconnect else R.string.connect)
        status.text = when {
            running -> "VPN connected"
            BpVpnService.lastError().isNotEmpty() -> "Failed: " + BpVpnService.lastError()
            else -> "Idle"
        }
    }

    private fun connect() {
        if (prefs.state.active !in prefs.state.nodes.indices) {
            Toast.makeText(this, "Add and select a node first", Toast.LENGTH_LONG).show()
            return
        }
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
        }
        startService(svc)
        refresh()
    }

    private fun disconnect() {
        startService(
            Intent(this, BpVpnService::class.java).apply { action = BpVpnService.ACTION_STOP }
        )
        refresh()
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQ_VPN) {
            if (resultCode == RESULT_OK) startVpn()
            else Toast.makeText(this, "VPN permission denied", Toast.LENGTH_LONG).show()
        }
    }

    companion object {
        private const val REQ_VPN = 1
        private const val TICK_MS = 1000L
    }
}
