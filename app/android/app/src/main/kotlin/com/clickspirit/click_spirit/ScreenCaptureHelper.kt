package com.clickspirit.click_spirit

import android.media.projection.MediaProjection

/**
 * Placeholder for a future MediaProjection capture path.
 *
 * TODO: request a MediaProjection token, create a virtual display plus ImageReader,
 * and hand frames to the image-match clicker. The manifest already declares
 * FOREGROUND_SERVICE and FOREGROUND_SERVICE_MEDIA_PROJECTION for that work.
 * The service must be a foreground service with type mediaProjection before
 * the virtual display is created.
 */
class ScreenCaptureHelper {
    var projection: MediaProjection? = null
        private set

    fun attach(projection: MediaProjection) {
        this.projection = projection
    }

    fun release() {
        projection?.stop()
        projection = null
    }
}
