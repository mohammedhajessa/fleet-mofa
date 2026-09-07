package com.fleetdm.agent

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import java.io.File

class ApkCommandWorker(appContext: Context, workerParams: WorkerParameters) : CoroutineWorker(appContext, workerParams) {
    override suspend fun doWork(): Result {
        val commands = ApiClient.getPendingApkCommands().getOrElse { error ->
            FleetLog.e(TAG, "Unable to fetch pending APK commands", error)
            return Result.retry()
        }.commands

        commands.forEach { command ->
            val apkFile = File(applicationContext.cacheDir, "mofa-apks/${command.commandId}.apk")
            ApiClient.downloadApk(command.commandId, command.sha256, apkFile).fold(
                onSuccess = {
                    val ready = command.status != ApkCommandStatus.PENDING ||
                        ApiClient.updateApkCommandStatus(command.commandId, ApkCommandStatus.DOWNLOADED).isSuccess
                    if (ready) showInstallNotification(command, apkFile)
                },
                onFailure = { error ->
                    FleetLog.e(TAG, "Unable to download APK command ${command.commandId}", error)
                    ApiClient.updateApkCommandStatus(
                        command.commandId,
                        ApkCommandStatus.FAILED,
                        error.message ?: "APK download failed",
                    )
                },
            )
        }
        return Result.success()
    }

    private fun showInstallNotification(command: PendingApkCommand, apkFile: File) {
        val manager = applicationContext.getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "Company app updates", NotificationManager.IMPORTANCE_HIGH),
        )
        val intent = Intent(applicationContext, InstallApkActivity::class.java).apply {
            putExtra(InstallApkActivity.EXTRA_COMMAND_ID, command.commandId)
            putExtra(InstallApkActivity.EXTRA_PACKAGE_NAME, command.packageName)
            putExtra(InstallApkActivity.EXTRA_VERSION_CODE, command.versionCode)
            putExtra(InstallApkActivity.EXTRA_APK_PATH, apkFile.absolutePath)
        }
        val pendingIntent = PendingIntent.getActivity(
            applicationContext,
            command.commandId.hashCode(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val notification = NotificationCompat.Builder(applicationContext, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle("${command.name} ${command.versionName}")
            .setContentText("Tap to install this company app")
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()
        manager.notify(command.commandId.hashCode(), notification)
    }

    companion object {
        const val WORK_NAME = "mofa-apk-command-check"
        private const val CHANNEL_ID = "mofa-apk-installs"
        private const val TAG = "fleet-apk-worker"
    }
}
