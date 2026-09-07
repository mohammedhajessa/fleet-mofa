package com.fleetdm.agent

import android.os.Build
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import java.net.URI
import java.util.UUID
import kotlinx.coroutines.launch

@Composable
fun EnrollmentScreen(modifier: Modifier = Modifier) {
    var serverUrl by remember { mutableStateOf("") }
    var enrollSecret by remember { mutableStateOf("") }
    var isSubmitting by remember { mutableStateOf(false) }
    var errorMessage by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()

    Column(
        modifier = modifier.padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Connect this device to Fleet", style = MaterialTheme.typography.headlineSmall)
        Text("Enter the Fleet server URL and an enrollment secret supplied by your IT administrator.")
        OutlinedTextField(
            value = serverUrl,
            onValueChange = { serverUrl = it; errorMessage = null },
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSubmitting,
            singleLine = true,
            label = { Text("Fleet server URL") },
            placeholder = { Text("https://fleet.example.com") },
        )
        OutlinedTextField(
            value = enrollSecret,
            onValueChange = { enrollSecret = it; errorMessage = null },
            modifier = Modifier.fillMaxWidth(),
            enabled = !isSubmitting,
            singleLine = true,
            label = { Text("Enrollment secret") },
            visualTransformation = PasswordVisualTransformation(),
        )
        errorMessage?.let { Text(it, color = MaterialTheme.colorScheme.error) }
        Button(
            onClick = {
                val normalizedUrl = serverUrl.trim().trimEnd('/')
                val validationError = validateEnrollmentInput(normalizedUrl, enrollSecret)
                if (validationError != null) {
                    errorMessage = validationError
                    return@Button
                }
                isSubmitting = true
                scope.launch {
                    ApiClient.setEnrollmentCredentials(
                        enrollSecret = enrollSecret.trim(),
                        hardwareUUID = UUID.randomUUID().toString(),
                        serverUrl = normalizedUrl,
                        computerName = "${Build.BRAND} ${Build.MODEL}",
                    )
                    ApiClient.enroll().onFailure { error ->
                        errorMessage = error.message ?: "Fleet enrollment failed"
                    }
                    isSubmitting = false
                }
            },
            enabled = !isSubmitting,
        ) {
            if (isSubmitting) CircularProgressIndicator(modifier = Modifier.padding(end = 8.dp), strokeWidth = 2.dp)
            Text(if (isSubmitting) "Connecting…" else "Connect")
        }
    }
}

internal fun validateEnrollmentInput(serverUrl: String, enrollSecret: String): String? {
    if (serverUrl.isBlank()) return "Fleet server URL is required"
    if (enrollSecret.isBlank()) return "Enrollment secret is required"
    val uri = runCatching { URI(serverUrl) }.getOrNull() ?: return "Fleet server URL is invalid"
    if (uri.scheme !in setOf("https", "http") || uri.host.isNullOrBlank()) {
        return "Fleet server URL must start with https:// or http://"
    }
    return null
}
