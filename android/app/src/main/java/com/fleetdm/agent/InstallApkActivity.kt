package com.fleetdm.agent

import android.content.Intent
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.FileProvider
import androidx.lifecycle.lifecycleScope
import java.io.File
import kotlinx.coroutines.launch

class InstallApkActivity : ComponentActivity() {
    private lateinit var commandId: String
    private lateinit var packageNameToVerify: String
    private lateinit var apkPath: String
    private var versionCodeToVerify: Long = 0

    private val unknownSourcesLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) {
        if (packageManager.canRequestPackageInstalls()) {
            launchPackageInstaller()
        } else {
            reportAndFinish(ApkCommandStatus.FAILED, "Permission to install company APKs was not granted")
        }
    }

    private val installerLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) {
        val installedVersion = runCatching {
            packageManager.getPackageInfo(packageNameToVerify, 0).longVersionCode
        }.getOrNull()
        if (installedVersion != null && installedVersion >= versionCodeToVerify) {
            reportAndFinish(ApkCommandStatus.INSTALLED)
        } else {
            reportAndFinish(ApkCommandStatus.FAILED, "Installation was cancelled or failed")
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        commandId = intent.getStringExtra(EXTRA_COMMAND_ID).orEmpty()
        packageNameToVerify = intent.getStringExtra(EXTRA_PACKAGE_NAME).orEmpty()
        apkPath = intent.getStringExtra(EXTRA_APK_PATH).orEmpty()
        versionCodeToVerify = intent.getLongExtra(EXTRA_VERSION_CODE, 0)
        if (commandId.isBlank() || packageNameToVerify.isBlank() || versionCodeToVerify <= 0 || !File(apkPath).isFile) {
            finish()
            return
        }
        val archiveInfo = packageManager.getPackageArchiveInfo(apkPath, 0)
        if (archiveInfo == null || archiveInfo.packageName != packageNameToVerify || archiveInfo.longVersionCode != versionCodeToVerify) {
            reportAndFinish(ApkCommandStatus.FAILED, "APK package or version does not match the uploaded metadata")
            return
        }
        if (!packageManager.canRequestPackageInstalls()) {
            unknownSourcesLauncher.launch(
                Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, android.net.Uri.parse("package:$packageName")),
            )
            return
        }
        launchPackageInstaller()
    }

    private fun launchPackageInstaller() {
        lifecycleScope.launch {
            val result = ApiClient.updateApkCommandStatus(commandId, ApkCommandStatus.AWAITING_USER)
            if (result.isFailure) {
                finish()
                return@launch
            }
            try {
                val apkUri = FileProvider.getUriForFile(this@InstallApkActivity, "$packageName.files", File(apkPath))
                val installIntent = Intent(Intent.ACTION_VIEW).apply {
                    setDataAndType(apkUri, "application/vnd.android.package-archive")
                    addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                }
                installerLauncher.launch(installIntent)
            } catch (error: Exception) {
                ApiClient.updateApkCommandStatus(
                    commandId,
                    ApkCommandStatus.FAILED,
                    error.message ?: "Android package installer could not be opened",
                )
                finish()
            }
        }
    }

    private fun reportAndFinish(status: ApkCommandStatus, detail: String? = null) {
        lifecycleScope.launch {
            ApiClient.updateApkCommandStatus(commandId, status, detail)
            finish()
        }
    }

    companion object {
        const val EXTRA_COMMAND_ID = "command_id"
        const val EXTRA_PACKAGE_NAME = "package_name"
        const val EXTRA_VERSION_CODE = "version_code"
        const val EXTRA_APK_PATH = "apk_path"
    }
}
