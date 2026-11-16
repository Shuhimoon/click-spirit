package com.clickspirit.click_spirit

import android.app.Activity
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.MediaRecorder
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.SystemClock
import android.util.Log
import kotlin.math.max
import kotlin.math.roundToInt

/**
 * 錄影轉腳本 on Android: a foreground service of type mediaProjection that
 * records the screen to an mp4 with MediaRecorder through a VirtualDisplay.
 * The user grants MediaProjection consent in [MainActivity] first.
 */
class ScreenRecordService : Service() {
    private val main = Handler(Looper.getMainLooper())
    private var projection: MediaProjection? = null
    private var display: VirtualDisplay? = null
    private var recorder: MediaRecorder? = null
    private var path: String = ""

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) {
            stopSelf()
            return START_NOT_STICKY
        }
        startInForeground()
        val code = intent.getIntExtra(EXTRA_CODE, Activity.RESULT_CANCELED)
        @Suppress("DEPRECATION")
        val data: Intent? = if (Build.VERSION.SDK_INT >= 33) {
            intent.getParcelableExtra(EXTRA_DATA, Intent::class.java)
        } else {
            intent.getParcelableExtra(EXTRA_DATA)
        }
        path = intent.getStringExtra(EXTRA_PATH) ?: ""
        val error = try {
            begin(code, data)
            null
        } catch (t: Throwable) {
            Log.e(TAG, "cannot start screen recording", t)
            release()
            t.message ?: t.toString()
        }
        val cb = pendingStart
        pendingStart = null
        if (error == null) {
            instance = this
            cb?.invoke(SystemClock.uptimeMillis(), null)
        } else {
            cb?.invoke(0L, error)
            stopSelf()
        }
        return START_NOT_STICKY
    }

    private fun startInForeground() {
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL, "螢幕錄影", NotificationManager.IMPORTANCE_LOW),
            )
            Notification.Builder(this, CHANNEL)
        } else {
            @Suppress("DEPRECATION")
            Notification.Builder(this)
        }
        val notification = builder
            .setContentTitle("點擊精靈正在錄影")
            .setContentText("回到點擊精靈按「停止錄影」產生腳本")
            .setSmallIcon(android.R.drawable.presence_video_online)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun begin(code: Int, data: Intent?) {
        require(data != null && code == Activity.RESULT_OK) { "沒有取得錄影授權" }
        require(path.isNotEmpty()) { "沒有影片路徑" }
        val mpm = getSystemService(Context.MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        val mp = mpm.getMediaProjection(code, data) ?: error("無法取得 MediaProjection")
        projection = mp
        mp.registerCallback(object : MediaProjection.Callback() {
            override fun onStop() {
                main.post { release() }
            }
        }, main)

        val (sw, sh) = realScreenSize(this)
        // Encoders reject very large or odd sizes: cap the long side at 1280.
        val scale = minOf(1.0, 1280.0 / max(sw, sh))
        val vw = ((sw * scale).roundToInt() / 16) * 16
        val vh = ((sh * scale).roundToInt() / 16) * 16
        val rec = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) MediaRecorder(this) else {
            @Suppress("DEPRECATION")
            MediaRecorder()
        }
        rec.setVideoSource(MediaRecorder.VideoSource.SURFACE)
        rec.setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
        rec.setVideoEncoder(MediaRecorder.VideoEncoder.H264)
        rec.setVideoSize(vw, vh)
        rec.setVideoFrameRate(30)
        rec.setVideoEncodingBitRate(5_000_000)
        rec.setOutputFile(path)
        rec.prepare()
        recorder = rec
        display = mp.createVirtualDisplay(
            "click-spirit-recording",
            vw,
            vh,
            resources.displayMetrics.densityDpi,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            rec.surface,
            null,
            null,
        )
        rec.start()
    }

    /** Stops and finalises the mp4. Returns an error message or null. */
    fun finish(): String? {
        var err: String? = null
        try {
            recorder?.stop()
        } catch (t: Throwable) {
            err = "影片結束時出錯：${t.message}"
        }
        release()
        return err
    }

    private fun release() {
        try {
            recorder?.reset()
            recorder?.release()
        } catch (ignored: Throwable) {
        }
        recorder = null
        display?.release()
        display = null
        try {
            projection?.stop()
        } catch (ignored: Throwable) {
        }
        projection = null
        if (instance === this) instance = null
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE)
        } else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
        stopSelf()
    }

    override fun onDestroy() {
        if (recorder != null || projection != null) release()
        super.onDestroy()
    }

    companion object {
        private const val TAG = "ClickSpiritRec"
        private const val CHANNEL = "click_spirit_recording"
        private const val NOTIFICATION_ID = 4711
        const val EXTRA_CODE = "code"
        const val EXTRA_DATA = "data"
        const val EXTRA_PATH = "path"

        @Volatile
        var instance: ScreenRecordService? = null
            private set

        /** Called on the main thread with (startUptime, error). */
        var pendingStart: ((Long, String?) -> Unit)? = null

        /** Real (full) screen size in pixels; the space taps use. */
        fun realScreenSize(context: Context): Pair<Int, Int> {
            val dm = context.getSystemService(Context.DISPLAY_SERVICE) as DisplayManager
            val m = android.util.DisplayMetrics()
            val d = dm.getDisplay(android.view.Display.DEFAULT_DISPLAY)
            if (d == null) {
                val r = context.resources.displayMetrics
                return Pair(r.widthPixels, r.heightPixels)
            }
            @Suppress("DEPRECATION")
            d.getRealMetrics(m)
            return Pair(m.widthPixels, m.heightPixels)
        }
    }
}
