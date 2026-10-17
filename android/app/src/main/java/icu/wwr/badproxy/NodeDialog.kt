package icu.wwr.badproxy

import android.app.AlertDialog
import android.content.Context
import android.view.LayoutInflater
import android.widget.ArrayAdapter
import android.widget.EditText
import android.widget.Spinner

// NodeDialog is the add/edit form. Fields mirror the desktop GUI's node
// schema one-to-one so exported JSON stays interchangeable. Protocol and
// transport are closed lists: the engine only speaks these, and transport
// never offers a plaintext option.
class NodeDialog(
    private val ctx: Context,
    private val onSave: (Node) -> Unit,
) {
    private val view = LayoutInflater.from(ctx).inflate(R.layout.dialog_node, null)
    private val dialog = AlertDialog.Builder(ctx)
        .setTitle(R.string.node_title)
        .setView(view)
        .setPositiveButton(android.R.string.ok) { _, _ -> save() }
        .setNegativeButton(android.R.string.cancel, null)
        .create()

    private val name = view.findViewById<EditText>(R.id.name_input)
    private val protocol = view.findViewById<Spinner>(R.id.protocol_spinner)
    private val server = view.findViewById<EditText>(R.id.server_input)
    private val port = view.findViewById<EditText>(R.id.port_input)
    private val secret = view.findViewById<EditText>(R.id.secret_input)
    private val transport = view.findViewById<Spinner>(R.id.transport_spinner)
    private val path = view.findViewById<EditText>(R.id.path_input)
    private val authority = view.findViewById<EditText>(R.id.authority_input)
    private val padding = view.findViewById<EditText>(R.id.padding_input)

    init {
        fillChoices(protocol, listOf("vless", "btp"), "vless")
        fillChoices(transport, listOf("tls", "xhttp"), "tls")
        port.setText("443")
    }

    fun show() = dialog.show()

    fun show(node: Node) {
        name.setText(node.name)
        selectByValue(protocol, listOf("vless", "btp"), node.protocol)
        server.setText(node.server)
        port.setText(node.port)
        secret.setText(node.secret)
        selectByValue(transport, listOf("tls", "xhttp"), node.transport)
        path.setText(node.path)
        authority.setText(node.authority)
        padding.setText(node.padding)
        dialog.show()
    }

    private fun save() {
        val n = Node(
            name = name.text.toString().trim().ifEmpty { server.text.toString().trim() },
            protocol = protocol.selectedItem.toString(),
            server = server.text.toString().trim(),
            port = port.text.toString().trim().ifEmpty { "443" },
            secret = secret.text.toString().trim(),
            transport = transport.selectedItem.toString(),
            path = path.text.toString().trim(),
            authority = authority.text.toString().trim(),
            padding = padding.text.toString().trim(),
        )
        if (n.server.isEmpty() || n.secret.isEmpty()) return
        onSave(n)
    }

    private fun fillChoices(spinner: Spinner, values: List<String>, def: String) {
        spinner.adapter = ArrayAdapter(
            ctx, android.R.layout.simple_spinner_dropdown_item, values,
        )
        selectByValue(spinner, values, def)
    }

    private fun selectByValue(spinner: Spinner, values: List<String>, value: String) {
        val i = values.indexOf(value)
        if (i >= 0) spinner.setSelection(i)
    }
}
