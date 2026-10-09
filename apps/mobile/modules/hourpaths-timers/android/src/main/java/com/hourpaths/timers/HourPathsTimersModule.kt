package com.hourpaths.timers

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.Uri
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition

class HourPathsTimersModule : Module() {
  private val channel = "hourpaths-running-timers"
  private val notificationId = 184732

  override fun definition() = ModuleDefinition {
    Name("HourPathsTimers")

    AsyncFunction("clear") {
      val context = appContext.reactContext ?: throw IllegalStateException("timer_context_unavailable")
      val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
      manager.cancel(channel, notificationId)
    }

    AsyncFunction("replace") { title: String, names: List<String>, startedAt: Double, channelName: String ->
      val context = appContext.reactContext ?: throw IllegalStateException("timer_context_unavailable")
      require(names.isNotEmpty() && startedAt.isFinite() && startedAt >= 0)
      val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
      if (!manager.areNotificationsEnabled()) return@AsyncFunction
      manager.createNotificationChannel(NotificationChannel(channel, channelName, NotificationManager.IMPORTANCE_LOW).apply {
        setSound(null, null)
        enableVibration(false)
        setShowBadge(false)
      })
      val intent = context.packageManager.getLaunchIntentForPackage(context.packageName)
        ?: throw IllegalStateException("timer_launch_unavailable")
      intent.action = Intent.ACTION_VIEW
      intent.data = Uri.parse("hourpaths:///home")
      intent.addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP)
      val open = PendingIntent.getActivity(context, notificationId, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
      val expanded = Notification.InboxStyle()
      names.forEach { expanded.addLine(it) }
      val notification = Notification.Builder(context, channel)
        .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
        .setContentTitle(title)
        .setContentText(names.joinToString(" · "))
        .setStyle(expanded)
        .setWhen(startedAt.toLong())
        .setShowWhen(true)
        .setUsesChronometer(true)
        .setOngoing(true)
        .setOnlyAlertOnce(true)
        .setAutoCancel(false)
        .setCategory(Notification.CATEGORY_STOPWATCH)
        .setVisibility(Notification.VISIBILITY_PRIVATE)
        .setContentIntent(open)
        .build()
      manager.notify(channel, notificationId, notification)
    }
  }
}
