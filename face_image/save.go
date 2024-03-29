package face_image

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bookun/face-collector/entity"
	"github.com/bookun/face-collector/util"
	"github.com/disintegration/imaging"
	"gocv.io/x/gocv"
)

type parameter struct {
	name   string
	angle  float64
	blur   float64
	width  int
	height int
}

func getClassifier(path string) (gocv.CascadeClassifier, error) {
	classifier := gocv.NewCascadeClassifier()
	if !classifier.Load(path) {
		return gocv.CascadeClassifier{}, fmt.Errorf("failed to read cascade file")
	}
	return classifier, nil
}

func createImage(baseImage *image.RGBA, param parameter) (gocv.Mat, error) {
	rotateImg := imaging.Rotate(baseImage, param.angle, color.NRGBA{0, 0, 0, 0})
	blurImg := imaging.Blur(rotateImg, param.blur)
	resultMat, err := gocv.ImageToMatRGBA(blurImg)
	if err != nil {
		return gocv.Mat{}, err
	}
	gocv.Resize(resultMat, &resultMat, image.Point{param.width, param.height}, 0, 0, gocv.InterpolationDefault)
	return resultMat, nil
}

func SaveFaceImages(ctx context.Context, imagePath string, op entity.Operation) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		width := *op.Width
		height := *op.Height
		outputDir := *op.OutputDir
		classifier, err := getClassifier(*op.CascadeClassifier)
		if err != nil {
			return err
		}
		defer classifier.Close()

		img := gocv.IMRead(imagePath, 1)
		defer img.Close()
		imgImg, err := img.ToImage()
		if err != nil {
			return err
		}
		rects := classifier.DetectMultiScaleWithParams(img, 1.30, 4, 0, image.Point{0, 0}, image.Point{10000, 10000})
		fileName := filepath.Base(imagePath)
		isJpeg, err := util.IsJPEG(imagePath)
		if err != nil {
			return err
		}
		if !isJpeg {
			return nil
		}
		for i, r := range rects {
			fName := strings.Replace(imagePath, *op.InputDir, "", -1)
			parts := strings.Split(fName, "/")
			dirName := strings.Join(parts[:len(parts)-1], "/")
			dirPath := filepath.Join(outputDir, dirName)
			if err := util.CreateDir(dirPath); err != nil {
				return err
			}
			newImg := image.NewRGBA(r)
			draw.Draw(newImg, newImg.Bounds(), imgImg, r.Min, draw.Over)

			params := []parameter{
				{name: "Original", angle: 0, blur: 0, width: width, height: height},
			}
			if *op.DataArguation {
				params = append(params,
					parameter{name: "Original", angle: 0, blur: 0, width: width, height: height},
					parameter{name: "Angle30Blur0", angle: 30, blur: 0, width: width, height: height},
					parameter{name: "Angle45Blur0", angle: 45, blur: 0, width: width, height: height},
					parameter{name: "Angle315Blur0", angle: 315, blur: 0, width: width, height: height},
					parameter{name: "Angle330Blur0", angle: 330, blur: 0, width: width, height: height},
					parameter{name: "Angle0Blur3", angle: 0, blur: 3, width: width, height: height},
				)
			}
			wg := &sync.WaitGroup{}
			for _, param := range params {
				go func(param parameter, dirPath, fileName string, newImg *image.RGBA) {
					wg.Add(1)
					mat, err := createImage(newImg, param)
					if err != nil {
						fmt.Printf("[ERROR] %v\n", err)
					}
					if *op.Gray {
						gocv.CvtColor(mat, &mat, gocv.ColorBGRToGray)
					}
					if !gocv.IMWrite(fmt.Sprintf("%s/%d_%s_%s", dirPath, i, param.name, fileName), mat) {
						fmt.Printf("[ERROR] write error: %s/%d_%s_%s", dirPath, i, param.name, fileName)
					}
					wg.Done()
				}(param, dirPath, fileName, newImg)
			}
			wg.Wait()
		}
		return nil
	}
}
