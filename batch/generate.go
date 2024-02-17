package batch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Project struct {
	ProjectName string `json:"projectName"`
	Pages       []Page `json:"pages"`
}

type Page struct {
	Title string `json:"title"`
	Lines []struct {
		Text string `json:"text"`
	} `json:"lines"`
}

type FAQ struct {
	Question  string `json:"question"`
	PageTitle string `json:"pageTitle"`
}

const QuestionTextPrefix = "? "

func BatchGenerateFAQ() {
	projectName := os.Getenv("SCRAPBOX_PROJECT_NAME")
	faqsFileName := os.Getenv("FAQS_FILE_NAME")
	dataDirPath, err := filepath.Abs(filepath.Join("..", "..", "data"))
	if err != nil {
		panic(err)
	}
	faqsFilePath := filepath.Join(dataDirPath, faqsFileName)

	titles, err := getPageTitles(projectName)
	if err != nil {
		panic(err)
	}

	var faqs []FAQ
	for _, title := range titles {
		pageFaqs, err := convertPageToFAQs(projectName, title)
		if err != nil {
			panic(err)
		}
		faqs = append(faqs, pageFaqs...)
	}

	err = storageFaqs(faqs, faqsFilePath)
	if err != nil {
		panic(err)
	}
	fmt.Println("generate faqs successfully!")
}

func getPageTitles(projectName string) ([]string, error) {
	res, err := http.Get(fmt.Sprintf("https://scrapbox.io/api/pages/%s", projectName))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := json.Unmarshal(body, &project); err != nil {
		return nil, err
	}

	var titles []string
	for _, page := range project.Pages {
		titles = append(titles, page.Title)
	}
	return titles, nil
}

func convertPageToFAQs(projectName, pageTitle string) ([]FAQ, error) {
	res, err := http.Get(fmt.Sprintf("https://scrapbox.io/api/pages/%s/%s", projectName, pageTitle))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}

	var faqs []FAQ
	for _, line := range page.Lines[1:] {
		if strings.HasPrefix(line.Text, QuestionTextPrefix) {
			questionText := strings.TrimPrefix(line.Text, QuestionTextPrefix)
			questions := convertTextToQuestions(questionText)
			for _, question := range questions {
				faqs = append(faqs, FAQ{Question: question, PageTitle: pageTitle})
			}
		}
	}
	return faqs, nil
}

func convertTextToQuestions(text string) []string {
	re := regexp.MustCompile(`\(([^()]+)\)`)
	matches := re.FindAllString(text, -1)
	if matches == nil {
		return []string{text}
	}

	var optionsList [][]string
	for _, match := range matches {
		options := strings.Split(match[1:len(match)-1], "|")
		optionsList = append(optionsList, options)
	}

	combinations := generateCombinations(optionsList)
	var questions []string
	for _, combination := range combinations {
		tempText := text
		for _, option := range combination {
			tempText = re.ReplaceAllString(tempText, option)
		}
		questions = append(questions, tempText)
	}
	return questions
}

func generateCombinations(optionsList [][]string) [][]string {
	var result [][]string
	var combine func(list [][]string, depth int, current []string)
	combine = func(list [][]string, depth int, current []string) {
		if depth == len(list) {
			result = append(result, append([]string(nil), current...))
			return
		}
		for _, option := range list[depth] {
			combine(list, depth+1, append(current, option))
		}
	}
	combine(optionsList, 0, []string{})
	return result
}

func storageFaqs(faqs []FAQ, filePath string) error {
	data, err := json.Marshal(faqs)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}
	return nil
}
