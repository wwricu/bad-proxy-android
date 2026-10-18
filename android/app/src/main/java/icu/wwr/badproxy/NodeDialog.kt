package icu.wwr.badproxy

import android.app.AlertDialog
import android.content.Context
import android.view.LayoutInflater
import android.view.View
import android.widget.AdapterView
import android.widget.ArrayAdapter
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.Spinner
import android.widget.TextView

// NodeDialog is the add/edit form. Fields and their visibility rules mirror
// the desktop GUI one-to-one (state.go visibleNodeFields, node.js), so
// exported JSON stays interchangeable.
class NodeDialog(
    private val ctx: Context,
    private val onSave: (Node) -> Unit,
) {
    private val view = LayoutInflater.from(ctx).inflate(R.layout.dialog_node, null)
    private val dialog = AlertDialog.Builder(ctx)
        .setTitle(R.string.node_title)
        .setView(view)
        .setPositiveButton(android.R.string.ok, null)
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
    private val msg = view.findViewById<TextView>(R.id.form_msg)

    init {
        fillChoices(protocol, PROTOCOLS, "vless")
        fillChoices(transport, TRANSPORTS, "xhttp")
        port.setText("443")
        protocol.onItemSelectedListener = spinnerListener()
        transport.onItemSelectedListener = spinnerListener()
        applyVisibility()
    }

    private fun spinnerListener() = object : AdapterView.OnItemSelectedListener {
        override fun onItemSelected(p: AdapterView<*>?, v: View?, pos: Int, id: Long) = applyVisibility()
        override fun onNothingSelected(p: AdapterView<*>?) = applyVisibility()
    }

    // Same rule set as the desktop app: secret is not a socks field, authority
    // belongs to both encrypted hops, path/padding only to xhttp.
    private fun applyVisibility() {
        val proto = protocol.selectedItem?.toString() ?: "vless"
        val trans = transport.selectedItem?.toString() ?: "tls"
        setRow(R.id.row_secret, proto != "socks")
        setRow(R.id.row_authority, trans == "tls" || trans == "xhttp")
        setRow(R.id.row_path, trans == "xhttp")
        setRow(R.id.row_padding, trans == "xhttp")
    }

    private fun setRow(id: Int, visible: Boolean) {
        view.findViewById<LinearLayout>(id).visibility = if (visible) View.VISIBLE else View.GONE
    }

    fun show() {
        dialog.show()
        wireSave()
    }

    fun show(node: Node) {
        name.setText(node.name)
        selectByValue(protocol, PROTOCOLS, node.protocol)
        server.setText(node.server)
        port.setText(node.port)
        secret.setText(node.secret)
        selectByValue(transport, TRANSPORTS, node.transport)
        path.setText(node.path)
        authority.setText(node.authority)
        padding.setText(node.padding)
        applyVisibility()
        dialog.show()
        wireSave()
    }

    // Overriding the positive button after show() keeps the dialog open when
    // validation fails, like the desktop form's inline message does.
    private fun wireSave() {
        dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener { save() }
    }

    private fun save() {
        val n = Node(
            name = name.text.toString().trim(),
            protocol = protocol.selectedItem.toString(),
            server = server.text.toString().trim(),
            port = port.text.toString().trim(),
            secret = secret.text.toString().trim(),
            transport = transport.selectedItem.toString(),
            path = path.text.toString().trim(),
            authority = authority.text.toString().trim(),
            padding = padding.text.toString().trim(),
        )
        if (n.name.isEmpty() || n.server.isEmpty() || n.port.isEmpty()) {
            msg.text = ctx.getText(R.string.form_required)
            return
        }
        onSave(n)
        dialog.dismiss()
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

    companion object {
        val PROTOCOLS = listOf("vless", "btp", "trojan", "socks")
        val TRANSPORTS = listOf("tls", "xhttp")
    }
}
