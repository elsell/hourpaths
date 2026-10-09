import { cacheDirectory, deleteAsync } from 'expo-file-system/legacy';
import { launchImageLibraryAsync } from 'expo-image-picker';
import { PictureFailure, validatePictureBytes } from '@hourpaths/client-core';

let picking = false;
/** The image-picker module is used only here; never touch the user's library. */
export async function clearProfilePictureCache(): Promise<void> {
  if (cacheDirectory) await deleteAsync(`${cacheDirectory}ImagePicker/`, { idempotent: true });
}

/** System UI owns selection/cropping; no broad library permission or camera access. */
export async function pickProfilePicture(): Promise<string | null> {
  if (picking) throw new PictureFailure();
  picking = true;
  try {
    await clearProfilePictureCache();
    const result = await launchImageLibraryAsync({
      mediaTypes: ['images'], allowsEditing: true, aspect: [1, 1],
      base64: true, exif: false, quality: 1,
    });
    if (result.canceled) return null;
    const asset = result.assets[0];
    if (!asset?.base64) throw new PictureFailure('invalid');
    return validatePictureBytes(asset.base64);
  } finally {
    try { await clearProfilePictureCache(); }
    finally { picking = false; }
  }
}
