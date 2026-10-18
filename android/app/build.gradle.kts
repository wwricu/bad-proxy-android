plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "icu.wwr.badproxy"
    compileSdk = 36

    defaultConfig {
        applicationId = "icu.wwr.badproxy"
        minSdk = 24
        targetSdk = 33 // pre-34: no foreground-service-type declaration needed
        versionCode = 1
        versionName = "0.0.1-beta-1"
    }

    signingConfigs {
        // CI passes the upload key through env; without it the release build
        // falls back to the debug key so `./gradlew assembleRelease` still
        // works offline. A debug key differs per machine, so releases built
        // that way cannot upgrade each other.
        if (System.getenv("BP_KEYSTORE") != null) {
            create("release") {
                storeFile = file(System.getenv("BP_KEYSTORE"))
                storePassword = System.getenv("BP_KEYSTORE_PASSWORD")
                keyAlias = "badproxy"
                keyPassword = System.getenv("BP_KEYSTORE_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("release")
                ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
    kotlinOptions {
        jvmTarget = "11"
    }
}

dependencies {
    implementation(files("libs/vpn.aar"))
}
