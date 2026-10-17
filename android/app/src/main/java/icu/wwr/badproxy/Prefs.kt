package icu.wwr.badproxy

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File

data class Node(
    var name: String = "",
    var protocol: String = "vless",
    var server: String = "",
    var port: String = "443",
    var secret: String = "",
    var transport: String = "tls",
    var path: String = "",
    var authority: String = "",
    var padding: String = "",
)

class State {
    val nodes = mutableListOf<Node>()
    var active = -1
    var routingMode = "global"
}

// Prefs persists the app state as the same JSON schema the Go engine and the
// desktop GUI use, so node exports stay interchangeable.
class Prefs(ctx: Context) {

    private val sp = ctx.getSharedPreferences("state", Context.MODE_PRIVATE)
    val state = State()

    init {
        sp.getString("json", null)?.let { raw ->
            try {
                val o = JSONObject(raw)
                val arr = o.optJSONArray("nodes") ?: JSONArray()
                for (i in 0 until arr.length()) {
                    val n = arr.getJSONObject(i)
                    state.nodes.add(
                        Node(
                            name = n.optString("name"),
                            protocol = n.optString("protocol", "vless"),
                            server = n.optString("server"),
                            port = n.optString("port", "443"),
                            secret = n.optString("secret"),
                            transport = n.optString("transport", "tls"),
                            path = n.optString("path"),
                            authority = n.optString("authority"),
                            padding = n.optString("padding"),
                        )
                    )
                }
                state.active = o.optInt("active", if (state.nodes.isEmpty()) -1 else 0)
                state.routingMode = o.optString("routing_mode", "global")
            } catch (_: Exception) {
                // a corrupt state file must not brick the app; start clean
            }
        }
        if (state.nodes.isNotEmpty() && state.active !in state.nodes.indices) {
            state.active = 0
        }
    }

    fun save() {
        val arr = JSONArray()
        for (n in state.nodes) {
            arr.put(
                JSONObject()
                    .put("name", n.name).put("protocol", n.protocol)
                    .put("server", n.server).put("port", n.port)
                    .put("secret", n.secret).put("transport", n.transport)
                    .put("path", n.path).put("authority", n.authority)
                    .put("padding", n.padding)
            )
        }
        val o = JSONObject()
            .put("nodes", arr)
            .put("active", state.active)
            .put("routing_mode", state.routingMode)
        sp.edit().putString("json", o.toString()).apply()
    }

    // ensureRules unpacks the bundled routing dataset once; the engine reads
    // it from filesDir at every connect.
    fun ensureRules(ctx: Context) {
        val f = File(ctx.filesDir, "rules.dat")
        if (!f.exists()) {
            ctx.assets.open("rules.dat").use { input ->
                f.outputStream().use { input.copyTo(it) }
            }
        }
    }

    fun vpnConfigJson(filesDir: File): String {
        val o = JSONObject(vpnConfigObject())
        o.put("rules_path", File(filesDir, "rules.dat").absolutePath)
        // The engine's stdout never reaches logcat and TMPDIR is not writable
        // by the app, so the log path is decided here, in the one place that
        // knows the real private data dir.
        o.put("log_path", File(filesDir, "bad-proxy.log").absolutePath)
        return o.toString()
    }

    private fun vpnConfigObject(): String {
        val arr = JSONArray()
        for (n in state.nodes) {
            arr.put(
                JSONObject()
                    .put("name", n.name).put("protocol", n.protocol)
                    .put("server", n.server).put("port", n.port)
                    .put("secret", n.secret).put("transport", n.transport)
                    .put("path", n.path).put("authority", n.authority)
                    .put("padding", n.padding)
            )
        }
        return JSONObject()
            .put("nodes", arr)
            .put("active", state.active)
            .put("routing_mode", state.routingMode)
            .toString()
    }
}
