package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yamato0211/tsumaziro-faq-server/db/model"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/db"
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

func BatchGenerateFAQ(db *db.DB, ctx context.Context) error {
	var accounts []*model.Account
	if err := db.DB.NewSelect().Model((*model.Account)(nil)).Scan(ctx, &accounts); err != nil {
		errors.WithStack(err)
	}
	fmt.Println("accounts")
	fmt.Println(accounts)

	for _, account := range accounts {
		titles, err := getPageTitles(account.ProjectID)
		if err != nil {
			return errors.WithStack(err)
		}

		var faqs []FAQ
		for _, title := range titles {
			pageFaqs, err := convertPageToFAQs(account.ProjectID, title)
			if err != nil {
				return errors.WithStack(err)
			}
			faqs = append(faqs, pageFaqs...)
		}
		marshaled, err := json.Marshal(faqs)
		if err != nil {
			return errors.WithStack(err)
		}
		account.Faqs = marshaled
		account.UpdatedAt = time.Now()
		if _, err = db.DB.NewUpdate().Model(account).WherePK().Exec(ctx); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
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
